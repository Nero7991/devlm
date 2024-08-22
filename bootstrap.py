import abc
import time
from typing import Dict, Any
import anthropic
import datetime
import os
import requests
import json
from pyjson5 import load as json5_load
from io import StringIO
import re
import shutil
import time
from functools import wraps
import copy
try:
    import anthropic
    from anthropic import AnthropicVertex
except ImportError:
    print("Error: anthropic package is not installed. Please run: pip install anthropic[vertex]")
    sys.exit(1)

try:
    from google.auth import default
    from google.auth.transport.requests import Request
    from google.oauth2.credentials import Credentials
    from google.cloud import aiplatform
except ImportError:
    print("Error: Google Cloud packages are not installed. Please run:")
    print("pip install --upgrade google-auth google-auth-oauthlib google-auth-httplib2 google-cloud-aiplatform")
    sys.exit(1)

# Set up Google Cloud credentials
credentials, project = default()
if not credentials.valid:
    if credentials.expired and credentials.refresh_token:
        credentials.refresh(Request())
    else:
        raise ValueError("Invalid Google Cloud credentials. Please run 'gcloud auth application-default login'")

# Set up Google Cloud credentials
try:
    credentials, project = default()
    if not credentials.valid:
        if credentials.expired and credentials.refresh_token:
            credentials.refresh(Request())
        else:
            raise ValueError("Invalid Google Cloud credentials. Please run 'gcloud auth application-default login'")
except Exception as e:
    print(f"Error setting up Google Cloud credentials: {str(e)}")
    print("Please make sure you have run 'gcloud auth application-default login' and have the necessary permissions.")
    sys.exit(1)

# Replace with your actual API key
API_KEY = "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"
# API_KEY = "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"
# API_KEY = "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" # CU

class LLMError(Exception):
    def __init__(self, error_type, message):
        self.error_type = error_type
        self.message = message
        super().__init__(f"{error_type}: {message}")

class LLMInterface(abc.ABC):
    @abc.abstractmethod
    def generate_response(self, prompt: str, max_tokens: int) -> str:
        pass

class AnthropicLLM(LLMInterface):
    def __init__(self, client):
        self.client = client

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        while True:
            try:
                response = self.client.messages.create(
                    model="claude-3-5-sonnet-20240620",
                    max_tokens=max_tokens,
                    messages=[
                        {"role": "user", "content": prompt}
                    ]
                )
                return response.content[0].text if response.content else ""

            except anthropic.APIError as e:
                if hasattr(e, 'status_code'):
                    error_data = e.response.json() if hasattr(e, 'response') else {}
                    error = error_data.get('error', {})
                    error_type = error.get('type', 'unknown_error')
                    error_message = error.get('message', str(e))
                    
                    if self._handle_error(error_type, error_message):
                        continue  # Retry after handling the error
                    else:
                        raise LLMError(error_type, error_message)
                else:
                    print(f"API error: {str(e)}")
                    raise

            except Exception as e:
                # Handle any unexpected exceptions
                print(f"Unexpected error: {str(e)}")
                raise

    def _handle_error(self, error_type, error_message):
        if error_type == 'rate_limit_error':
            if 'daily rate limit' in error_message.lower():
                self._wait_until_midnight()
            else:
                self._handle_rate_limit(error_message)
            return True
        elif error_type == 'overloaded_error':
            self._handle_overloaded()
            return True
        elif error_type == 'invalid_request_error' and 'credit balance is too low' in error_message.lower():
            self._handle_credit_issue()
            return True
        elif error_type == 'internal_server_error':
            if retries < self.max_retries:
                wait_time = self._calculate_wait_time(retries)
                print(f"Internal server error. Retrying in {wait_time} seconds...")
                time.sleep(wait_time)
                return True
            else:
                return False
        return False

    def _calculate_wait_time(self, retries):
        return self.base_delay * (2 ** retries) + random.uniform(0, 1)

    def _wait_until_midnight(self):
        now = datetime.datetime.now()
        tomorrow = now.replace(hour=0, minute=0, second=0, microsecond=0) + datetime.timedelta(days=1)
        wait_time = (tomorrow - now).total_seconds()
        print(f"Daily rate limit reached. Waiting until midnight ({tomorrow.strftime('%Y-%m-%d %H:%M:%S')})...")
        time.sleep(wait_time)


    def _handle_rate_limit(self, error_message):
        wait_time = 60  # Default to 60 seconds
        # Try to extract wait time from error message if available
        try:
            wait_time = int(error_message.split("try again in ")[1].split(" ")[0])
        except:
            pass
        print(f"Rate limit exceeded. Retrying in {wait_time} seconds...")
        time.sleep(wait_time)

    def _handle_overloaded(self):
        wait_time = 60
        print(f"API temporarily overloaded. Retrying in {wait_time} seconds...")
        time.sleep(wait_time)

    def _handle_credit_issue(self):
        print("Your account has insufficient credit. Please add credit to your account.")
        input("Press Enter once you've added credit to continue, or Ctrl+C to exit...")


class VertexAILLM(LLMInterface):
    def __init__(self, project_id, region):
        self.project_id = project_id
        self.region = region
        self.client = AnthropicVertex(region=region, project_id=project_id)
        self.max_retries = 5
        self.retry_delay = 1  # Start with 1 second delay

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        messages = [{"role": "user", "content": prompt}]
        
        for attempt in range(self.max_retries):
            try:
                response = self.client.messages.create(
                    model="claude-3-5-sonnet@20240620",
                    max_tokens=max_tokens,
                    messages=messages
                )
                return response.content[0].text if response.content else ""
            except Exception as e:
                if attempt < self.max_retries - 1:
                    print(f"Error occurred: {str(e)}. Retrying in {self.retry_delay} seconds...")
                    time.sleep(self.retry_delay)
                    self.retry_delay *= 2  # Exponential backoff
                else:
                    print(f"Max retries reached. Error: {str(e)}")
                    user_input = input("Do you want to try again? (yes/no): ").lower()
                    if user_input == 'yes':
                        self.retry_delay = 1  # Reset delay
                        continue
                    else:
                        raise

        raise Exception("Failed to generate response after multiple attempts")

    def _handle_error(self, error_type, error_message):
        print(f"Vertex AI error: {error_type} - {error_message}")
        return False

def get_llm_client(provider: str = "anthropic") -> LLMInterface:
    if provider == "anthropic":
        return AnthropicLLM(anthropic.Anthropic(api_key=API_KEY))
    elif provider == "vertex_ai":
        # Replace with your actual Google Cloud project ID and region
        project_id = "gen-lang-client-0101555698"
        region = "us-east5"
        return VertexAILLM(project_id, region)
    else:
        raise ValueError(f"Unsupported LLM provider: {provider}")

# Update the global llm_client variable
llm_client = get_llm_client("vertex_ai")  # or "anthropic" based on your preference

# llm_client = get_llm_client()

# Initialize the Anthropic client
# client = anthropic.Anthropic(api_key=API_KEY)

TECHNICAL_BRIEF_FILE = "project_technical_brief.json"

def wait_until_midnight():
    now = datetime.now()
    tomorrow = now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(days=1)
    wait_time = (tomorrow - now).total_seconds()
    print(f"Rate limit exceeded. Waiting until midnight ({tomorrow.strftime('%Y-%m-%d %H:%M:%S')})...")
    time.sleep(wait_time)

def handle_low_credit_balance():
    print("Error: Your credit balance is too low to access the Claude API.")
    print("Please go to Plans & Billing to upgrade or purchase credits.")
    input("Press Enter when you have added credits to continue, or Ctrl+C to exit...")

def get_last_processed_file():
    if os.path.exists(TECHNICAL_BRIEF_FILE):
        with open(TECHNICAL_BRIEF_FILE, 'r') as f:
            brief = json.load(f)
        last_processed = None
        last_iteration = 0
        for dir_entry in brief["directories"]:
            for file_entry in dir_entry["files"]:
                if file_entry.get("last_updated_iteration", 0) > last_iteration:
                    last_processed = os.path.join(dir_entry["path"].lstrip('/'), file_entry["name"])
                    last_iteration = file_entry["last_updated_iteration"]
        return last_processed
    return None

from functools import wraps

def retry_on_overload(max_retries=3, initial_delay=1, backoff_factor=2):
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            delay = initial_delay
            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    if attempt == max_retries - 1:
                        raise
                    if isinstance(e, anthropic.RateLimitError) or "rate limit" in str(e).lower():
                        print(f"Rate limit exceeded. Retrying in {delay} seconds...")
                        time.sleep(delay)
                        delay *= backoff_factor
                    else:
                        raise
            return None  # This line should never be reached due to the raise in the loop
        return wrapper
    return decorator

def get_project_structure():
    if os.path.exists("project_structure.json"):
        with open("project_structure.json", "r") as f:
            return json.load(f)
    else:
        return {
            "golang": ["main.go", "api_gateway.go", "backend_service.go", "action_executor.go", "code_runner.go"],
            "python": ["llm_service.py", "models.py", "utils.py"],
            "docker": ["Dockerfile.golang", "Dockerfile.python"],
            "config": ["github_config.yaml", "api_endpoints.json"],
            "root": ["dev.txt", "README.md", "docker-compose.yml"]
        }

def update_directory_summary(brief, directory_path):
    path_parts = directory_path.split(os.sep)
    current_dir = brief["directories"]
    for part in path_parts:
        if part:
            current_dir = current_dir["directories"][part]

    # Check if all files and subdirectories are processed
    all_processed = all(f["status"] in ["done", "in_progress"] for f in current_dir["files"]) and \
                    all(subdir in brief["directory_summaries"] for subdir in current_dir["directories"])

    if all_processed:
        directory_summary_prompt = f"""Please provide a concise summary of the following directory based on its files, functions, and subdirectories:

{json.dumps(current_dir, indent=2)}

The summary should be a brief overview of the directory's purpose and main components. It should be useful for an AI when updating or creating new files in this directory or its subdirectories. Include key information about:

1. The overall purpose of this directory
2. Main functionalities implemented in its files
3. Important relationships or dependencies between files or subdirectories
4. Any design patterns or architectural decisions evident from the structure
5. Key interfaces or APIs exposed by this directory's components

If this directory only contains subdirectories, focus on the overall structure and purpose of these subdirectories.

Limit your response to 200 words.
"""
        directory_summary = llm_client.generate_response(directory_summary_prompt, 2000)
        brief["directory_summaries"][directory_path] = directory_summary.strip()

        # Recursively update parent directory summaries
        parent_dir = os.path.dirname(directory_path)
        if parent_dir:
            update_directory_summary(brief, parent_dir)


def review_project_structure(project_summary):
    current_structure = get_project_structure()
    prompt = f"""As an experienced software developer, please review and suggest improvements to the following project structure for our LLM-based Software Developer Project. Consider best practices, scalability, and maintainability. Suggest a new structure if needed, explaining your reasoning.

Current Project Structure:
{json.dumps(current_structure, indent=2)}

Project Summary:
{project_summary}

Please provide your suggested project structure as a JSON object, following this format:
{{
    "directory_name": ["file1.ext", "file2.ext"],
    "another_directory": ["file3.ext"]
}}

Important: Make sure to include appropriate file extensions for all files in your suggested structure.

Also, include sample configuration files (e.g., GitHub URL, API endpoints) in your suggested structure. The actual content for these will be provided by the user later.

Also, provide a brief explanation of your suggested changes.
"""

    try:
        response_text = llm_client.generate_response(prompt, 4000)
        
        json_match = re.search(r'\{[\s\S]*\}', response_text)
        if json_match:
            suggested_structure = json.loads(json_match.group(0))
            explanation = response_text.split(json_match.group(0))[-1].strip()
            return suggested_structure, explanation
        else:
            raise ValueError("No JSON object found in the response")
    except Exception as e:
        print(f"Error reviewing project structure: {str(e)}")
        return None, None

def create_project_structure(structure):
    def create_files(path, items):
        if isinstance(items, list):
            for item in items:
                file_path = os.path.join(path, item)
                os.makedirs(os.path.dirname(file_path), exist_ok=True)
                open(file_path, 'w').close()
        elif isinstance(items, dict):
            for subdir, subitems in items.items():
                subpath = os.path.join(path, subdir)
                create_files(subpath, subitems)

    for directory, items in structure.items():
        if directory == "":
            create_files(".", items)
        else:
            create_files(directory, items)

def remove_old_structure(preserve_files):
    for root, dirs, files in os.walk(".", topdown=False):
        for name in files:
            if name not in preserve_files:
                os.remove(os.path.join(root, name))
        for name in dirs:
            try:
                os.rmdir(os.path.join(root, name))
            except OSError:
                pass  # Directory not empty, will be removed in next iterations

def initialize_technical_brief(structure):
    if os.path.exists(TECHNICAL_BRIEF_FILE):
        print("Technical brief already exists. Checking progress...")
        return check_progress(structure)

    brief = {
        "project": "LLM-based Software Developer Project",
        "directory_summaries": {},
        "directories": {}
    }

    def process_directory(items):
        dir_entry = {
            "files": [],
            "directories": {}
        }
        for name, content in items.items():
            if isinstance(content, str):  # It's a file
                if name != "bootstrap.py":
                    dir_entry["files"].append({"name": name, "functions": [], "status": "not_started"})
            elif isinstance(content, dict):  # It's a directory
                dir_entry["directories"][name] = process_directory(content)
        return dir_entry

    brief["directories"] = process_directory(structure)
    
    with open(TECHNICAL_BRIEF_FILE, 'w') as f:
        json.dump(brief, f, indent=4)
    
    return brief

def check_progress(structure):
    with open(TECHNICAL_BRIEF_FILE, 'r') as f:
        brief = json.load(f)
    
    def update_directory_progress(brief_dir, structure_dir, current_path=""):
        for file_name, file_info in structure_dir.items():
            if isinstance(file_info, str):  # It's a file
                file_path = os.path.join(current_path, file_name)
                file_entry = next((f for f in brief_dir.get("files", []) if f["name"] == file_name), None)
                if file_entry is None:
                    file_entry = {"name": file_name, "functions": [], "status": "not_started"}
                    brief_dir.setdefault("files", []).append(file_entry)
                
                if os.path.exists(file_path):
                    with open(file_path, 'r') as f:
                        content = f.read()
                    if content.strip():
                        file_entry["status"] = "in_progress"
                    else:
                        file_entry["status"] = "not_started"
                else:
                    file_entry["status"] = "not_started"
            elif isinstance(file_info, dict):  # It's a directory
                new_path = os.path.join(current_path, file_name)
                brief_subdir = brief_dir.get("directories", {}).setdefault(file_name, {"files": [], "directories": {}})
                update_directory_progress(brief_subdir, file_info, new_path)
    
    update_directory_progress(brief["directories"], structure)
    
    with open(TECHNICAL_BRIEF_FILE, 'w') as f:
        json.dump(brief, f, indent=4)
    
    return brief

def update_technical_brief(file_path, content, iteration):
    with open(TECHNICAL_BRIEF_FILE, 'r') as f:
        brief = json.load(f)
    
    file_entry = find_file_entry(brief["directories"], file_path)
    
    if file_entry is None:
        file_entry = {"name": os.path.basename(file_path), "functions": [], "status": "not_started"}
        update_file_entry(brief["directories"], file_path, file_entry)

    # Store the original file entry for comparison
    original_file_entry = copy.deepcopy(file_entry)

    prompt = f"""Based on the following file content, please generate a complete and valid JSON object for the technical brief of the file {os.path.basename(file_path)}. The brief should include a summary of the file's purpose and a list of functions with their inputs, outputs, and a brief summary. Also, include a "todo" field for each function if there's anything that needs to be completed or improved.

File content:
{content}

Output format:
{{
    "name": "{os.path.basename(file_path)}",
    "summary": "File summary",
    "status": "in_progress",
    "functions": [
        {{
            "name": "function_name",
            "inputs": ["param1", "param2"],
            "input_types": ["type1", "type2"],
            "outputs": ["result"],
            "output_types": ["result_type"],
            "summary": "Brief description of the function",
            "todo": "Optional: any additional information or tasks to be completed"
        }}
    ]
}}

Important: Ensure that the JSON is complete, properly formatted, and enclosed in triple backticks. Do not include any text outside the JSON object.
"""

    try:
        response_text = llm_client.generate_response(prompt, 4000)
        
        json_match = re.search(r'```(?:json)?\n([\s\S]*?)\n```', response_text, re.DOTALL)
        if json_match:
            json_str = json_match.group(1)
        else:
            json_str = response_text

        try:
            brief_content = json.loads(json_str)
        except json.JSONDecodeError:
            brief_content = json5_load(StringIO(json_str))

        file_entry.update(brief_content)
        file_entry["last_updated_iteration"] = iteration
        
        def is_todo_empty(todo):
            if not todo:
                return True
            todo_lower = str(todo).lower().strip()
            return todo_lower in ['', 'none', 'n/a', 'na', 'null']

        # Update the status based on the presence of non-empty todos
        if any(not is_todo_empty(func.get("todo")) for func in file_entry["functions"]):
            file_entry["status"] = "in_progress"
        else:
            file_entry["status"] = "done"

        # Update root directory summary if this is a root-level file
        if len(file_path.split(os.sep)) == 1:
            update_root_directory_summary(brief)
        else:
            # Update directory summary for non-root files
            update_directory_summary(brief, os.path.dirname(file_path))

        # Compare the updated file entry with the original
        if file_entry != original_file_entry:
            print(f"File entry for {os.path.basename(file_path)} has been updated:")
            
        else:
            print(f"Warning: File entry for {os.path.basename(file_path)} remained unchanged after update.")

    except Exception as e:
        print(f"Error updating technical brief for {file_path}: {str(e)}")
        file_entry.update({
            "name": os.path.basename(file_path),
            "summary": f"Error generating technical brief for {os.path.basename(file_path)}",
            "functions": [],
            "status": "error",
            "last_updated_iteration": iteration
        })

    # Save the updated brief immediately after updating a file
    save_technical_brief(brief)

    return brief

def save_technical_brief(brief):
    temp_file = TECHNICAL_BRIEF_FILE + ".temp"
    with open(temp_file, 'w') as f:
        json.dump(brief, f, indent=4)
    os.replace(temp_file, TECHNICAL_BRIEF_FILE)
    print(f"Technical brief saved to {TECHNICAL_BRIEF_FILE}")

    # Verify that the file was actually updated
    with open(TECHNICAL_BRIEF_FILE, 'r') as f:
        saved_brief = json.load(f)
    if saved_brief != brief:
        print("Warning: The saved technical brief does not match the in-memory version.")
        print("In-memory version:", brief)
        print("Saved version:", saved_brief)

def update_root_directory_summary(brief):
    root_files = brief["directories"].get("files", [])
    
    prompt = f"""Please provide a concise summary of the root directory based on the following files:

{json.dumps(root_files, indent=2)}

The summary should focus on the purpose and content of these root-level files, their relationships, and their role in the project structure. Do not include information about subdirectories, as they have their own summaries.

Limit your response to 200 words.
"""

    try:
        root_summary = llm_client.generate_response(prompt, 2000)
        brief["directory_summaries"]["."] = root_summary.strip()
    except Exception as e:
        print(f"Error generating root directory summary: {str(e)}")
        brief["directory_summaries"]["."] = "Error generating root directory summary"

def get_context_for_file(file_path, brief):
    path_parts = file_path.split(os.sep)
    current_dir = brief["directories"]
    context = {
        "directory_summaries": {},
        "current_directory": {}
    }
    
    # Special handling for root directory files
    if len(path_parts) == 1:
        context["directory_summaries"]["."] = brief.get("directory_summaries", {}).get(".", "No summary available for root directory")
        context["current_directory"] = {
            "files": current_dir.get("files", [])
        }
        return context

    # Build up the context with relevant directory summaries
    current_path = ""
    for part in path_parts[:-1]:
        current_path = os.path.join(current_path, part)
        if current_path in brief.get("directory_summaries", {}):
            context["directory_summaries"][current_path] = brief["directory_summaries"][current_path]
        if part in current_dir.get("directories", {}):
            current_dir = current_dir["directories"][part]
    
    context["current_directory"] = current_dir
    return context

def get_file_content(file_path, project_summary, technical_brief, previous_content="", iteration=1, max_iterations=5):
    prompt = f"""Based on the following project summary, technical brief, and previous content, please generate or update the content for the file {file_path}. Include necessary imports, basic structure, and functions or classes as appropriate. Ensure the generated content is consistent with the existing project structure and previously generated files. Focus on completing the todos for each function.

This is iteration {iteration} out of a maximum of {max_iterations}. You will have multiple iterations to complete this file, so you can focus on improving specific parts in each iteration.

Project Summary:
{project_summary}

Technical Brief:
{json.dumps(technical_brief, indent=2)}

Previous Content:
{previous_content}

Please provide the complete content for the file {file_path}, addressing any todos and improving the code as needed. Your output should be valid code ONLY, without any explanations or comments outside the code itself. If you need to include any explanations, please do so as comments within the code.

For configuration files, please use placeholder values that the user can easily identify and replace later.
"""

    try:
        response_text = llm_client.generate_response(prompt, 4000)
        
        # Check if the response starts with a code block
        if response_text.strip().startswith("```"):
            # Extract code from the code block
            code_match = re.search(r'```(?:\w+)?\n([\s\S]*?)\n```', response_text)
            if code_match:
                return code_match.group(1).strip()
        
        # If no code block is found, return the entire response
        return response_text.strip()
    except Exception as e:
        print(f"Error generating content for {file_path}: {str(e)}")
        return None

def get_processed_files():
    if os.path.exists(TECHNICAL_BRIEF_FILE):
        with open(TECHNICAL_BRIEF_FILE, 'r') as f:
            brief = json.load(f)
        processed_files = set()
        for dir_entry in brief["directories"]:
            for file_entry in dir_entry["files"]:
                if file_entry.get("last_updated_iteration", 0) > 0:
                    processed_files.add(os.path.join(dir_entry["path"].lstrip('/'), file_entry["name"]))
        return processed_files
    return set()

def read_project_structure():
    with open('project_structure.json', 'r') as f:
        return json.load(f)

ALLOWED_COMMANDS = [
    'python',
    'go run',
    'go test',
    'docker build',
    'docker run',
    'pip install',
    'go mod tidy',
    # Add more commands as needed
]

import subprocess

def execute_command(command):
    try:
        result = subprocess.run(command, shell=True, check=True, capture_output=True, text=True)
        return result.stdout
    except subprocess.CalledProcessError as e:
        return e.output

def modify_file(file_path, content):
    with open(file_path, 'w') as f:
        f.write(content)

def read_file(file_path):
    with open(file_path, 'r') as f:
        return f.read()

def test_and_debug_mode(llm_client):
    project_structure = read_project_structure()
    while True:
        prompt = f"""
        Based on the following project structure, suggest a test to run or a file to inspect:
        {json.dumps(project_structure, indent=2)}

        You can use the following commands: {', '.join(ALLOWED_COMMANDS)}
        
        If you want to inspect a file, reply with "INSPECT: <file_path>".
        If you want to modify a file, reply with "MODIFY: <file_path>".
        If you're done testing, reply with "DONE".

        What would you like to do?
        """
        
        response = llm_client.generate_response(prompt, 2000)
        
        if response.startswith("INSPECT:"):
            file_path = response.split(":")[1].strip()
            file_content = read_file(file_path)
            print(f"Content of {file_path}:\n{file_content}")
        
        elif response.startswith("MODIFY:"):
            file_path = response.split(":")[1].strip()
            current_content = read_file(file_path)
            modification_prompt = f"""
            Current content of {file_path}:
            {current_content}

            Please provide the updated content for this file:
            """
            new_content = llm_client.generate_response(modification_prompt, 4000)
            modify_file(file_path, new_content)
            print(f"Updated {file_path}")
        
        elif response.lower() == "done":
            break
        
        else:
            if any(cmd in response for cmd in ALLOWED_COMMANDS):
                output = execute_command(response)
                print(f"Command output:\n{output}")
            else:
                print("Invalid command or action.")

def find_file_entry(directories, file_path):
    path_parts = file_path.split(os.sep)
    current_dir = directories
    
    # Special handling for root directory files
    if len(path_parts) == 1:
        return next((f for f in current_dir.get("files", []) if f["name"] == path_parts[0]), None)
    
    for part in path_parts[:-1]:
        if part in current_dir.get("directories", {}):
            current_dir = current_dir["directories"][part]
        else:
            return None
    
    return next((f for f in current_dir.get("files", []) if f["name"] == path_parts[-1]), None)

def update_file_entry(directories, file_path, file_entry):
    path_parts = file_path.split(os.sep)
    current_dir = directories
    
    # Special handling for root directory files
    if len(path_parts) == 1:
        existing_entry = next((f for f in current_dir.get("files", []) if f["name"] == file_entry["name"]), None)
        if existing_entry:
            existing_entry.update(file_entry)
        else:
            current_dir.setdefault("files", []).append(file_entry)
        return

    for part in path_parts[:-1]:
        if part not in current_dir["directories"]:
            current_dir["directories"][part] = {"files": [], "directories": {}}
        current_dir = current_dir["directories"][part]
    
    existing_entry = next((f for f in current_dir["files"] if f["name"] == file_entry["name"]), None)
    if existing_entry:
        existing_entry.update(file_entry)
    else:
        current_dir["files"].append(file_entry)

def generate():
    with open('project_summary.md', 'r') as f:
        project_summary = f.read()

    current_structure = get_project_structure()

    user_input = input("Do you want to review and potentially update the project structure? (yes/no): ").lower()
    if user_input == 'yes':
        suggested_structure, explanation = review_project_structure(project_summary)

        if suggested_structure:
            print("Suggested project structure:")
            print(json.dumps(suggested_structure, indent=2))
            print("\nExplanation:")
            print(explanation)
            
            user_input = input("Do you want to apply the suggested structure? (yes/no): ").lower()
            if user_input == 'yes':
                # Backup the current structure
                if os.path.exists("project_structure.json"):
                    shutil.copy("project_structure.json", "project_structure_backup.json")
                
                # List of files to preserve
                preserve_files = ["bootstrap.py", "project_summary.md", "project_structure.json"]
                
                # Remove old structure
                remove_old_structure(preserve_files)
                
                # Save the new structure
                with open("project_structure.json", "w") as f:
                    json.dump(suggested_structure, f, indent=2)
                
                # Create new structure
                create_project_structure(suggested_structure)
                technical_brief = initialize_technical_brief(suggested_structure)
            else:
                technical_brief = initialize_technical_brief(current_structure)
        else:
            print("Couldn't get a suggested structure due to API overload. Using the current structure.")
            technical_brief = initialize_technical_brief(current_structure)
    else:
        technical_brief = initialize_technical_brief(current_structure)

    all_files = []
    def collect_files(directory):
        all_files.extend(directory["files"])
        for subdir in directory["directories"].values():
            collect_files(subdir)

    collect_files(technical_brief["directories"])
    
    current_iteration = min(file.get("last_updated_iteration", 0) for file in all_files)
    max_iterations = 5
    all_done = False

    # List of files to preserve and not regenerate
    preserve_files = ["bootstrap.py", "project_summary.md", "project_structure.json"]

    while not all_done and current_iteration < max_iterations:
        current_iteration += 1
        print(f"Starting iteration {current_iteration}")

        all_done = True

        structure = get_project_structure()
        files_to_process = []

        def collect_file_paths(directory, current_path=""):
            if isinstance(directory, list):
                return [os.path.join(current_path, item) for item in directory if isinstance(item, str) and item not in preserve_files]
            elif isinstance(directory, dict):
                files = []
                for subdir, items in directory.items():
                    new_path = os.path.join(current_path, subdir)
                    files.extend(collect_file_paths(items, new_path))
                return files
            return []

        files_to_process = collect_file_paths(structure)

        for file_path in files_to_process:
            if os.path.isdir(file_path):
                continue  # Skip directories

            with open(TECHNICAL_BRIEF_FILE, 'r') as f:
                technical_brief = json.load(f)

            file_entry = find_file_entry(technical_brief["directories"], file_path)
            
            if file_entry is None:
                print(f"Warning: File entry not found for {file_path}. Creating a new entry.")
                file_entry = {"name": os.path.basename(file_path), "functions": [], "status": "not_started", "last_updated_iteration": 0}
                update_file_entry(technical_brief["directories"], file_path, file_entry)

            print(f"Processing {file_path} (status: {file_entry.get('status', 'unknown')}), last updated: {file_entry.get('last_updated_iteration', 0)}")
            if file_entry.get("status") != "done":
                all_done = False

            # Process the file if it's not done or hasn't been updated in the current iteration
            if (file_entry.get("status") != "done") and (file_entry.get("last_updated_iteration", 0) < current_iteration):
                all_done = False
                
                # Handle root directory files differently
                if os.path.dirname(file_path) == '':
                    dir_to_create = '.'
                else:
                    dir_to_create = os.path.dirname(file_path)
                
                # Ensure the directory exists
                os.makedirs(dir_to_create, exist_ok=True)
                
                try:
                    with open(file_path, 'r') as f:
                        previous_content = f.read()
                except FileNotFoundError:
                    previous_content = ""
                
                try:
                    context = get_context_for_file(file_path, technical_brief)
                    content = get_file_content(file_path, project_summary, context, previous_content, current_iteration, max_iterations)
                    if content:
                        with open(file_path, 'w') as f:
                            f.write(content)
                        print(f"Updated {file_path}")
                        technical_brief = update_technical_brief(file_path, content, current_iteration)
                    else:
                        print(f"Failed to update {file_path}")
                        raise Exception(f"Failed to generate content for {file_path}")
                except Exception as e:
                    print(f"Error processing {file_path}: {str(e)}")
                    file_entry["status"] = "error"
                    file_entry["last_updated_iteration"] = current_iteration
                    update_file_entry(technical_brief["directories"], file_path, file_entry)
                    save_technical_brief(technical_brief)

            else:
                print(f"Skipping {file_path} - already processed or marked as done")

        print(f"Iteration {current_iteration} completed")
        time.sleep(1)  # Add a small delay to avoid rate limiting

    if all_done:
        print("All files are marked as done")
    else:
        print(f"Reached maximum iterations ({max_iterations}) or encountered errors")
        print("You can run the script again to continue from where it left off.")

def main():
    mode = input("Enter mode (generate/test): ").lower()
    
    if mode == "test":
        test_and_debug_mode(llm_client)
    elif mode == "generate":
        generate()
    else:
        print("Invalid mode. Please choose 'generate' or 'test'.")
        return
    


if __name__ == "__main__":
    main()