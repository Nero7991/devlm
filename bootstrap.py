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
import sys
from datetime import datetime
import shlex
import signal
import subprocess
import tempfile
import threading
import queue
import atexit
from typing import Optional
import psutil
import difflib

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
    def __init__(self, project_id: str, region: str, model: Optional[str] = None):
        self.project_id = project_id
        self.region = region
        self.client = AnthropicVertex(region=region, project_id=project_id)
        self.max_retries = 5
        self.retry_delay = 32  # Start with 32 seconds delay
        self.model = model or "claude-3-5-sonnet@20240620"  # Default model

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        messages = [{"role": "user", "content": prompt}]
        full_response = ""
        iteration = 0
        max_iterations = 2  # Limit the number of iterations to prevent infinite loops

        while iteration < max_iterations:
            for attempt in range(self.max_retries):
                try:
                    response = self.client.messages.create(
                        model=self.model,
                        max_tokens=max_tokens,
                        messages=messages
                    )
                    
                    # For debugging, print the response usage
                    if response.usage:
                        print(f"Response usage: {response.usage}")
                    
                    current_output = response.content[0].text if response.content else ""
                    full_response += current_output

                    # Check if we're close to the token limit (within 5%)
                    if response.usage and response.usage.output_tokens > 0.999 * max_tokens:
                        print("Output close to token limit. Continuing response...")
                        continuation_prompt = (
                            f"{prompt}\n\n"
                            f"Previous output (possibly incomplete):\n<<<START>>>{full_response}<<<END>>>\n\n"
                            "The previous response was very close to the output token limit and might not have completed. "
                            "Your previous output starts after the third greater than sign in <<<START>>> and ends at the character before the first less than sign in <<<END>>>. Please continue the output (including new line and tabs if needed), picking up where you left off without repeating information, your output will be appended without modification before first less than sign in <<<END>>>. Do not include anything other than the continuation of the output."
                        )
                        messages = [{"role": "user", "content": continuation_prompt}]
                        iteration += 1
                        break
                    else:
                        return full_response

                except Exception as e:
                    if attempt < self.max_retries - 1:
                        print(f"Error occurred: {str(e)}. Retrying in {self.retry_delay} seconds...")
                        time.sleep(self.retry_delay)
                        if self.retry_delay < 64:
                            self.retry_delay *= 2  # Exponential backoff
                    else:
                        print(f"Max retries reached. Error: {str(e)}")
                        user_input = input("Do you want to try again? (yes/no): ").lower()
                        if user_input == 'yes':
                            self.retry_delay = 32  # Reset delay
                            continue
                        else:
                            raise

            if iteration == max_iterations:
                print("Reached maximum number of continuation attempts.")
                break

        return full_response

    def switch_model(self, new_model: str):
        self.model = new_model
        print(f"Switched to model: {self.model}")

    def _handle_error(self, error_type, error_message):
        print(f"Vertex AI error: {error_type} - {error_message}")
        return False

def get_llm_client(provider: str = "anthropic", model: Optional[str] = None) -> LLMInterface:
    if provider == "anthropic":
        return AnthropicLLM(anthropic.Anthropic(api_key=API_KEY))
    elif provider == "vertex_ai":
        # Replace with your actual Google Cloud project ID and region
        project_id = "devlm-435701"
        region = "us-east5"
        return VertexAILLM(project_id, region, model)
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
        "directories": {
            "files": [],
            "directories": {}
        }
    }

    def process_directory(items):
        dir_entry = {
            "files": [],
            "directories": {}
        }
        if isinstance(items, list):
            for file in items:
                if file not in ["bootstrap.py", "project_structure.json", "project_summary.md"]:
                    dir_entry["files"].append({"name": file, "functions": [], "status": "not_started"})
        elif isinstance(items, dict):
            for name, content in items.items():
                if isinstance(content, list):  # It's a list of files
                    sub_dir = {
                        "files": [],
                        "directories": {}
                    }
                    for file in content:
                        if file not in ["bootstrap.py", "project_structure.json", "project_summary.md"]:
                            sub_dir["files"].append({"name": file, "functions": [], "status": "not_started"})
                    dir_entry["directories"][name] = sub_dir
                elif isinstance(content, dict):  # It's a subdirectory
                    dir_entry["directories"][name] = process_directory(content)
        return dir_entry

    for name, content in structure.items():
        if name == "":  # Root-level files
            for file in content:
                if file not in ["bootstrap.py", "project_structure.json", "project_summary.md"]:
                    brief["directories"]["files"].append({"name": file, "functions": [], "status": "not_started"})
        else:
            brief["directories"]["directories"][name] = process_directory(content)

    with open(TECHNICAL_BRIEF_FILE, 'w') as f:
        json.dump(brief, f, indent=4)

    save_technical_brief(brief)
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

def update_technical_brief(file_path, content, iteration, mode="generate", test_info=None):
    with open(TECHNICAL_BRIEF_FILE, 'r') as f:
        brief = json.load(f)
    
    file_entry = find_file_entry(brief["directories"], file_path)
    
    if file_entry is None:
        file_entry = {"name": os.path.basename(file_path), "functions": [], "status": "not_started"}
        update_file_entry(brief["directories"], file_path, file_entry)

    if mode == "generate":
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
            
            json_match = re.search(r'```(?:json)?\n([\s\S]*?)\n```', response_text)
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

            if any(not is_todo_empty(func.get("todo")) for func in file_entry["functions"]):
                file_entry["status"] = "in_progress"
            else:
                file_entry["status"] = "done"

        except Exception as e:
            print(f"Error updating technical brief for {file_path}: {str(e)}")
            file_entry.update({
                "name": os.path.basename(file_path),
                "summary": f"Error generating technical brief for {os.path.basename(file_path)}",
                "functions": [],
                "status": "error",
                "last_updated_iteration": iteration
            })

    elif mode == "test":
        if test_info:
            if "test_status" not in file_entry:
                file_entry["test_status"] = []
            file_entry["test_status"].append({
                "timestamp": datetime.now().isoformat(),
                "info": test_info
            })
        file_entry["last_updated_iteration"] = iteration
        file_entry["status"] = "tested"

    if len(file_path.split(os.sep)) == 1:
        update_root_directory_summary(brief)
    else:
        update_directory_summary(brief, os.path.dirname(file_path))

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

Project Structure:
{json.dumps(get_project_structure(), indent=2)}

Please provide the complete content for the file {file_path}, addressing any todos and improving the code as needed. Remember to correctly reference other packages, imports. Your output should be valid content for that file type, without any explanations or comments outside the content itself. If you need to include any explanations, please do so as comments within the code. Remember that you are directly writing to the file.

For configuration files, please use placeholder values that the user can easily identify and replace later.
"""

    try:
        response_text = llm_client.generate_response(prompt, 4000)
        
        # # Check if the response starts with a code block
        # if response_text.strip().startswith("```"):
        #     # Extract code from the code block
        #     code_match = re.search(r'```(?:\w+)?\n([\s\S]*?)\n```', response_text)
        #     if code_match:
        #         return code_match.group(1).strip()
        
        # # If no code block is found, return the entire response
        # return response_text.strip()

        # Extract code from the response
        code_content = extract_content(response_text, file_path)
        
        return code_content
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

def generate_project_structure(root_dir='.'):
    def create_structure(path):
        structure = {"": []}
        for item in os.listdir(path):
            if item == 'node_modules' or item.startswith('.'):
                continue
            full_path = os.path.join(path, item)
            if os.path.isfile(full_path):
                structure[""].append(item)
            elif os.path.isdir(full_path):
                structure[item] = create_structure(full_path)
        return structure

    return create_structure(root_dir)

def save_project_structure(structure):
    with open('project_structure.json', 'w') as f:
        json.dump(structure, f, indent=2)

def read_project_structure():
    if os.path.exists('project_structure.json'):
        with open('project_structure.json', 'r') as f:
            return json.load(f)
    return None

def inspect_file_with_approval(file_path):
    project_root = os.getcwd()  # Get the current working directory (project root)
    
    if not os.path.abspath(file_path).startswith(project_root):
        print(f"Warning: Attempting to access file outside project directory: {file_path}")
        approval = input("Do you approve this action? (yes/no): ").lower().strip()
        if approval != 'yes':
            print("Action not approved. Skipping file inspection.")
            return None
    
    try:
        if os.path.exists(file_path):
            if os.path.isfile(file_path):
                with open(file_path, 'rb') as f:
                    content = f.read(1024)  # Read first 1024 bytes
                if content.startswith(b'\x7fELF'):
                    return "This appears to be a binary executable file."
                else:
                    with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                        return f.read()
            elif os.path.isdir(file_path):
                return f"This is a directory. Contents: {os.listdir(file_path)}"
        else:
            return f"File or directory not found: {file_path}"
    except Exception as e:
        return f"Error inspecting file: {str(e)}"

ALLOWED_COMMANDS = [
    'python3',
    'go run',
    'go test',
    'docker build',
    'docker run',
    'pip3 install',
    'go mod tidy',
    'curl',
    'wget',
    'cd',
    # Add more commands as needed
]

APPROVAL_REQUIRED_COMMANDS = [
    'sudo apt install',
    './',
    # Add a raw command that requires approval
    'RAW: <raw_command>'
]

import subprocess

def require_approval(command):
    print(f"The following command requires your approval:")
    print(command)
    approval = input("Do you approve this command? (yes/no): ").lower().strip()
    return approval == 'yes'

def wait_for_user():
    print(f"Some error occurred. Please resolve the issue and press Enter to continue.")
    input()
    return True

TEST_PROGRESS_FILE = "test_progress.json"

def load_test_progress():
    if os.path.exists(TEST_PROGRESS_FILE):
        with open(TEST_PROGRESS_FILE, 'r') as f:
            return json.load(f)
    return {"completed_tests": [], "current_step": None}

def save_test_progress(progress):
    with open(TEST_PROGRESS_FILE, 'w') as f:
        json.dump(progress, f, indent=2)

def update_test_progress(completed_test=None, current_step=None):
    progress = load_test_progress()
    if completed_test:
        progress["completed_tests"].append(completed_test)
    if current_step:
        progress["current_step"] = current_step
    save_test_progress(progress)

running_processes = []

def check_all_processes():
    for process_info in running_processes[:]:  # Iterate over a copy of the list
        status, output = check_process_output(process_info["cmd"])
        if output:
            print(f"New output from '{process_info['cmd']}':\n{output}")
        if "has terminated" in status:
            print(status)
            # The process will be removed in check_process_output, so we don't need to do it here

def get_running_processes_info():
    return [{"cmd": p["cmd"]} for p in running_processes]

# Update the kill_all_processes function
def kill_all_processes():
    for process_info in running_processes:
        try:
            os.killpg(os.getpgid(process_info['process'].pid), signal.SIGTERM)
            print(f"Terminated process group: {process_info['cmd']}")
        except Exception as e:
            print(f"Error terminating process group {process_info['cmd']}: {str(e)}")
    running_processes.clear()

atexit.register(kill_all_processes)

process_initial_outputs = {}

import queue
import psutil

def parse_compound_command(command):
    parts = command.split('&&')
    cd_part = None
    run_part = None
    for part in parts:
        part = part.strip()
        if part.startswith('cd '):
            cd_part = part
        else:
            run_part = part
    return cd_part, run_part

def get_process_key(command):
    _, run_part = parse_compound_command(command)
    return run_part.split()[-1] if run_part else command.split()[-1]

def check_and_terminate_existing_process(command):
    cd_part, run_part = parse_compound_command(command)
    process_key = get_process_key(command)
    
    for proc in psutil.process_iter(['pid', 'name', 'cmdline']):
        try:
            cmdline = proc.info['cmdline']
            if cmdline and process_key in ' '.join(cmdline):
                print(f"Process '{process_key}' is already running. Terminating it.")
                parent = psutil.Process(proc.info['pid'])
                for child in parent.children(recursive=True):
                    child.terminate()
                parent.terminate()
                parent.wait(5)  # Wait for up to 5 seconds
                if parent.is_running():
                    print(f"Process didn't terminate gracefully. Forcing...")
                    parent.kill()
                    parent.wait(2)
                time.sleep(2)  # Wait for 2 seconds to ensure the port is released
                return True
        except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
            pass
    return False

def run_continuous_process(command):
    check_and_terminate_existing_process(command)

    cd_part, run_part = parse_compound_command(command)
    cwd = os.getcwd()
    
    if cd_part:
        target_dir = cd_part.split(None, 1)[1]
        os.chdir(target_dir)
    
    run_command = run_part if run_part else command

    try:
        # Start the new process in its own process group
        process = subprocess.Popen(shlex.split(run_command), stdout=subprocess.PIPE, stderr=subprocess.PIPE, 
                                   universal_newlines=True, preexec_fn=os.setpgrp)
        output_queue = queue.Queue()
        
        def enqueue_output(out, queue):
            for line in iter(out.readline, ''):
                queue.put(line)
            out.close()
        
        threading.Thread(target=enqueue_output, args=(process.stdout, output_queue), daemon=True).start()
        threading.Thread(target=enqueue_output, args=(process.stderr, output_queue), daemon=True).start()
        
        # Remove any old entries with the same command
        global running_processes
        running_processes = [p for p in running_processes if p['cmd'] != command]
        
        # Add the new process to the list
        running_processes.append({
            "cmd": command,
            "process": process,
            "queue": output_queue,
            "cwd": cwd,
            "run_command": run_command
        })
        
        # Wait for 10 seconds and collect initial output
        time.sleep(10)
        initial_output = ""
        while not output_queue.empty():
            initial_output += output_queue.get_nowait()
        
        # Keep only the last 2000 characters of the initial output
        initial_output = initial_output[-2000:]
        
        # Change back to the original directory
        os.chdir(cwd)
        
        return f"Started new process: {command}\nInitial output:\n{initial_output}"
    except PermissionError as e:
        error_output = f"PermissionError: {str(e)}\n"
        if run_command.endswith('.go'):
            suggestion = (
                "It seems you're trying to execute a Go file from the wrong directory."
                "To run a Go file, use the following format:\n"
                "cd /path/to/directory && go run filename.go\n"
                "For example: cd devlm-identity && go run cmd/api/main.go"
            )
            error_output += f"\nSuggestion: {suggestion}"
        return error_output
    except Exception as e:
        return f"Error executing command: {str(e)}"
    finally:
        os.chdir(cwd)

def check_process_output(command):
    process_key = get_process_key(command)
    for process_info in running_processes:
        if process_key in process_info["cmd"]:
            process = process_info["process"]
            output_queue = process_info["queue"]
            
            if process.poll() is not None:
                running_processes.remove(process_info)
                return f"Process '{command}' has terminated.", ""
            
            output = ""
            while not output_queue.empty():
                try:
                    output += output_queue.get_nowait()
                except queue.Empty:
                    break
            
            # Keep only the last 2000 characters
            output = output[-2000:]
            
            return f"Process '{command}' is running.", output
    
    return f"No running process found for command: {command}", ""

def restart_process(cmd):
    global running_processes
    process_key = get_process_key(cmd)
    process_found = False
    for process_info in running_processes:
        if process_key in process_info['cmd']:
            process_found = True
            try:
                os.killpg(os.getpgid(process_info['process'].pid), signal.SIGTERM)
                process_info['process'].wait(timeout=5)
            except Exception as e:
                print(f"Error terminating process {cmd}: {str(e)}")
            running_processes.remove(process_info)
            print(f"Process '{cmd}' has been terminated.")
            break
    
    if not process_found:
        print(f"Process '{cmd}' was not running.")
    
    # Start the process in the background
    print(f"Starting process '{cmd}' in the background.")
    output = run_continuous_process(cmd)
    
    return f"Process '{cmd}' has been restarted. Initial output:\n{output}"

command_decisions = {}

def execute_command(command, timeout=600):
    global command_decisions

    if command.upper().startswith("INDEF:"):
        cmd = command.split(":", 1)[1].strip()
        output = run_continuous_process(cmd)
        return output, True
    elif command.upper().startswith("CHECK:"):
        cmd = command.split(":", 1)[1].strip()
        output = check_process_output(cmd)
        return output, True
    elif command.startswith("RAW:"):
        raw_command = command[4:].strip()
        if not require_approval(raw_command):
            return "Command not approved by user.", False
        
        return execute_command_with_timeout(raw_command, timeout)
    else:
        if command.upper().startswith("RUN:"):
            command = command[4:].strip()

        if "go run" in command and command not in command_decisions:
            suggestion = (
                "This command appears to start a long-running process (like an API server). "
                "Consider using the INDEF action if it needs to run indefinitely. "
                "If you believe this process should complete quickly, you can use the RUN action again."
            )
            command_decisions[command] = "suggested_indef"
            return suggestion, True

        if command in command_decisions and command_decisions[command] == "suggested_indef":
            command_decisions[command] = "not_indefinite"
        
        if any(command.startswith(cmd) for cmd in APPROVAL_REQUIRED_COMMANDS):
            if not require_approval(command):
                return "Command not approved by user.", False
        
        return execute_command_with_timeout(command, timeout)

def execute_command_with_timeout(command, timeout):
    # Split the command into parts
    command_parts = command.split('&&')
    
    # Initialize variables
    current_dir = os.getcwd()
    output = ""
    return_code = 0

    try:
        for part in command_parts:
            part = part.strip()
            if part.startswith('cd '):
                # Change directory
                new_dir = part.split(None, 1)[1]
                try:
                    os.chdir(new_dir)
                    output += f"Changed directory to {new_dir}\n"
                except FileNotFoundError:
                    output += f"Error: Directory '{new_dir}' not found\n"
                    return_code = 1
                    break
            else:
                # Execute the command
                try:
                    process = subprocess.Popen(shlex.split(part), stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)
                    stdout, stderr = process.communicate(timeout=timeout)
                    return_code = process.returncode
                    output += f"Command: {part}\n"
                    output += f"STDOUT:\n{stdout}\n"
                    output += f"STDERR:\n{stderr}\n"
                    if return_code != 0:
                        output += f"Command failed with return code {return_code}\n"
                        break
                except subprocess.TimeoutExpired:
                    process.kill()
                    output += f"Command execution timed out after {timeout} seconds.\n"
                    return_code = -1
                    break
                except FileNotFoundError:
                    output += f"Error: Command '{part.split()[0]}' not found\n"
                    return_code = 1
                    break
    finally:
        # Always change back to the original directory
        os.chdir(current_dir)

    if return_code != 0:
        return output, False
    else:
        return output.strip(), True

def check_environment(command):
    print("Checking environment...")
    
    # Check current directory
    current_dir = os.getcwd()
    print(f"Current directory: {current_dir}")
    
    # Check if we're in the project root (you might want to adjust this check)
    if not os.path.exists("go.mod"):
        print("Warning: go.mod not found. We might not be in the project root.")
    
    # Extract the main command (e.g., 'go' from 'cd devlm-identity && go test ./...')
    main_command = command.split('&&')[-1].strip().split()[0]
    
    version_flag = "--version"
    if main_command == "python" or main_command == "python3":
        version_flag = "-V"
    elif "go" in main_command:
        version_flag = "version"
    
    version_command = f"{main_command} {version_flag}"
    output, success = execute_command_with_timeout(version_command, timeout=10)
    if success:
        print(f"{main_command.capitalize()} version: {output}")
        return success, "success"
    else:
        print(f"Error: {output}")
        return success, output

def modify_file(file_path, content):
    with open(file_path, 'w') as f:
        f.write(content)

def read_file(file_path):
    with open(file_path, 'r') as f:
        return f.read()

def load_technical_brief():
    if os.path.exists(TECHNICAL_BRIEF_FILE):
        with open(TECHNICAL_BRIEF_FILE, 'r') as f:
            return json.load(f)
    return {}

def get_file_technical_brief(technical_brief, file_path):
    def search_directories(directories, path_parts):
        if not path_parts:
            return None
        
        current_dir = path_parts[0]
        
        if current_dir in directories:
            if len(path_parts) == 1:
                for file in directories[current_dir].get("files", []):
                    if file["name"] == current_dir:
                        return file
            else:
                result = search_directories(directories[current_dir].get("directories", {}), path_parts[1:])
                if result is None:
                    for file in directories[current_dir].get("files", []):
                        if file["name"] == path_parts[-1]:
                            return file
                return result
        
        return None

    path_parts = file_path.split(os.sep)
    
    result = search_directories(technical_brief["directories"]["directories"], path_parts)
    
    if result is None:
        for file in technical_brief["directories"].get("files", []):
            if file["name"] == path_parts[-1]:
                return file
    
    return result

COMMAND_HISTORY_FILE = "command_history.json"

def save_command_history(command_history):
    with open(COMMAND_HISTORY_FILE, 'w') as f:
        json.dump(command_history, f, indent=2)

def load_command_history():
    if os.path.exists(COMMAND_HISTORY_FILE):
        with open(COMMAND_HISTORY_FILE, 'r') as f:
            return json.load(f)
    return []

def extract_content(response_text, file_path):
    # Print the response text (for debugging)  
    # print(f"Response text for {file_path}:\n{response_text}")

    file_extension = os.path.splitext(file_path)[1].lower()

    # File extensions where the LLM might respond with a code block
    code_block_extensions = [
        '.py', '.go', '.js', '.java', '.c', '.cpp', '.h', '.hpp', '.sh',
        '.html', '.css', '.sql', '.Dockerfile', '.makefile'
    ]

    # File extensions for plain text files
    plain_text_extensions = [
        '.md', '.txt', '.yml', '.yaml', '.ini', '.cfg', '.conf',
        '.gitignore', '.env', '.properties', '.log'
    ]

    if file_extension in code_block_extensions:
        # Check if the response contains a code block
        code_match = re.search(r'```(?:\w+)?\n([\s\S]*?)\n```', response_text)
        if code_match:
            return code_match.group(1).strip()
        else:
            # If no code block is found, return the entire response
            return response_text.strip()
    
    elif file_extension in plain_text_extensions or not file_extension:
        # For plain text files or files without extension, return the entire response
        return response_text.strip()

    elif file_extension == '.json':
        # For JSON files, attempt to parse and format the content
        try:
            parsed = json.loads(response_text)
            return json.dumps(parsed, indent=2)
        except json.JSONDecodeError:
            # If parsing fails, return the response as is
            return response_text.strip()

    else:
        # For any unknown file types, log a warning and return the response as is
        print(f"Warning: Unknown file extension '{file_extension}' for file '{file_path}'. Treating as plain text.")
        return response_text.strip()

def get_last_10_iterations(command_history):
    return command_history[-15:] if len(command_history) > 15 else command_history

llm_notes = {
    "general": "",
    "issues": [],
    "progress": [],
    "latest_changes": "",
    "next_steps": ""
}

MAX_NOTES_LENGTH = 2000

def update_notes(new_notes):
    global llm_notes
    try:
        updated_notes = json.loads(new_notes)
        for key in llm_notes.keys():
            if key in updated_notes:
                if isinstance(llm_notes[key], list):
                    llm_notes[key] = (llm_notes[key] + updated_notes[key])[-10:]  # Keep last 10 items
                else:
                    llm_notes[key] = updated_notes[key][-MAX_NOTES_LENGTH:]  # Truncate to max length
    except json.JSONDecodeError:
        print("Invalid JSON format for notes. Ignoring update.")

def add_line_numbers(content):
    """
    Add line numbers to the given content.
    
    Args:
    content (str): The content to add line numbers to.
    
    Returns:
    str: The content with line numbers added.
    """
    lines = content.split('\n')
    numbered_lines = [f"{i+1}:{line}" for i, line in enumerate(lines)]
    return '\n'.join(numbered_lines)

def remove_line_numbers(numbered_content):
    """
    Remove line numbers from the given numbered content.
    
    Args:
    numbered_content (str): The content with line numbers.
    
    Returns:
    str: The content without line numbers.
    """
    lines = numbered_content.split('\n')
    content_lines = [line.split(':', 1)[1] if ':' in line else line for line in lines]
    return '\n'.join(content_lines)

import tempfile
import subprocess
import os

def create_git_patch(file_path, changes):
    patch_lines = [
        f"diff --git a/{file_path} b/{file_path}",
        f"--- a/{file_path}",
        f"+++ b/{file_path}"
    ]
    hunk_lines = []
    current_line = 1
    hunk_start = None
    hunk_old_lines = 0
    hunk_new_lines = 0

    changes_list = sorted([c for c in changes.split('\n') if c.strip()], key=lambda x: int(x.split(':')[0].lstrip('+-')))

    for change in changes_list:
        if change.startswith('+'):
            line_num, content = change[1:].split(':', 1)
            line_num = int(line_num)
            if hunk_start is None:
                hunk_start = max(1, line_num)
            hunk_lines.append(f"+{content}")
            hunk_new_lines += 1
        elif change.startswith('-'):
            line_num = int(change[1:])
            if hunk_start is None:
                hunk_start = max(1, line_num)
            hunk_lines.append("-")
            hunk_old_lines += 1
        else:
            line_num, content = change.split(':', 1)
            line_num = int(line_num)
            if hunk_start is None:
                hunk_start = max(1, line_num)
            hunk_lines.append(f"-{content}")
            hunk_lines.append(f"+{content}")
            hunk_old_lines += 1
            hunk_new_lines += 1

        if hunk_start is not None and (len(hunk_lines) > 0 or line_num > current_line):
            patch_lines.append(f"@@ -{hunk_start},{hunk_old_lines} +{hunk_start},{hunk_new_lines} @@")
            patch_lines.extend(hunk_lines)
            hunk_lines = []
            hunk_start = None
            hunk_old_lines = 0
            hunk_new_lines = 0

        current_line = line_num

    if hunk_lines:
        patch_lines.append(f"@@ -{hunk_start},{hunk_old_lines} +{hunk_start},{hunk_new_lines} @@")
        patch_lines.extend(hunk_lines)

    return '\n'.join(patch_lines) + '\n'  # Add a newline at the end of the patch

def apply_git_patch(file_path, patch_content):
    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.patch') as temp_file:
        temp_file.write(patch_content)
        temp_file_path = temp_file.name

    try:
        # Add the file to the git index if it doesn't exist
        if not os.path.exists(file_path):
            with open(file_path, 'w') as f:
                f.write('')
            subprocess.run(['git', 'add', file_path], check=True)

        # Apply the patch
        result = subprocess.run(['git', 'apply', '--verbose', '--ignore-whitespace', '--unidiff-zero', '--reject', temp_file_path], 
                                capture_output=True, text=True)
        if result.returncode == 0:
            print(f"Successfully applied patch to {file_path}")
            return True
        else:
            print(f"Failed to apply patch: {result.stderr}")
            # Try to apply the patch with increased context
            result = subprocess.run(['git', 'apply', '--verbose', '--ignore-whitespace', '--unidiff-zero', '--reject', '-C3', temp_file_path],
                                    capture_output=True, text=True)
            if result.returncode == 0:
                print(f"Successfully applied patch to {file_path} with increased context")
                return True
            else:
                print(f"Failed to apply patch even with increased context: {result.stderr}")
                return False
    except subprocess.CalledProcessError as e:
        print(f"Error during patch application: {e}")
        return False
    finally:
        os.unlink(temp_file_path)


def parse_changes(changes_text):
    changes = []
    current_line = 1
    for line in changes_text.split('\n'):
        line = line.strip()
        if line and ':' in line:
            if line.startswith('+') or line.startswith('-'):
                changes.append(line)
            else:
                line_num, content = line.split(':', 1)
                if int(line_num) < current_line:
                    # This is likely a full rewrite, so we'll treat all lines as additions
                    changes.append(f"+{line_num}:{content}")
                else:
                    changes.append(line)
                current_line = int(line_num) + 1
        elif line.startswith('-'):
            # Handle standalone deletions (e.g., "-13")
            changes.append(f"{line}:")
    return changes

def apply_changes(current_content, changes):
    lines = current_content.split('\n') if current_content else []
    new_lines = []
    changes_dict = {}
    
    # Separate additions/modifications from deletions
    additions_modifications = []
    deletions = []
    
    for change in changes:
        line_num = int(change.split(':', 1)[0].lstrip('+-'))
        if change.startswith('-'):
            deletions.append(line_num)
        else:
            additions_modifications.append((line_num, change))
    
    # Sort additions and modifications
    additions_modifications.sort(key=lambda x: x[0])
    
    current_line = 1
    for line_num, change in additions_modifications:
        # Add any unchanged lines
        while current_line < line_num:
            if current_line not in deletions and current_line <= len(lines):
                new_lines.append(lines[current_line - 1])
            current_line += 1
        
        # Apply the change
        if change.startswith('+'):
            new_lines.append(change.split(':', 1)[1])
            if line_num not in deletions:
                current_line -= 1  # Don't increment current_line for additions
        else:
            new_lines.append(change.split(':', 1)[1])
        current_line += 1
    
    # Add any remaining unchanged lines
    while current_line <= len(lines):
        if current_line not in deletions:
            new_lines.append(lines[current_line - 1])
        current_line += 1
    
    return '\n'.join(new_lines)

def compare_and_write(file_path, new_content):
    try:
        with open(file_path, 'r') as f:
            old_content = f.read()
        
        if old_content != new_content:
            diff = list(difflib.unified_diff(old_content.splitlines(keepends=True), 
                                             new_content.splitlines(keepends=True), 
                                             fromfile='before', 
                                             tofile='after'))
            if diff:
                with open(file_path, 'w') as f:
                    f.write(new_content)
                print(f"Changes made to {file_path}:")
                print(''.join(diff))
                return True
            else:
                print(f"No actual changes to write in {file_path}")
                return False
        else:
            print(f"File {file_path} content is identical. No changes made.")
            return False
    except FileNotFoundError:
        return True

def update_project_structure(file_path):
    with open('project_structure.json', 'r') as f:
        project_structure = json.load(f)

    # Split the file path into components
    path_parts = file_path.split(os.sep)
    
    # Identify the top-level project (e.g., "devlm-identity" or "devlm-core")
    project = path_parts[0]
    
    # If the project doesn't exist in the structure, create it
    if project not in project_structure:
        project_structure[project] = {"": []}
    
    # Navigate through the directory structure
    current_level = project_structure[project]
    for part in path_parts[1:-1]:  # Exclude the last part (file name)
        if part not in current_level:
            current_level[part] = {"": []}
        current_level = current_level[part]
    
    # Add the file to the appropriate level
    file_name = path_parts[-1]
    if file_name not in current_level[""]:
        current_level[""].append(file_name)
        print(f"Updated project structure with new file: {file_path}")
    else:
        print(f"File {file_path} already exists in the project structure.")
    
    # Save the updated structure
    with open('project_structure.json', 'w') as f:
        json.dump(project_structure, f, indent=2)
    
# Add this new global variable
unchanged_files = {}

CHAT_FILE = "chat.txt"
last_chat_content = ""
chat_updated = False
chat_updated_iteration = 0

def ensure_chat_file_exists():
    if not os.path.exists(CHAT_FILE):
        with open(CHAT_FILE, 'w') as f:
            f.write("# Write your notes here. The script will pause after the current iteration when you save this file.\n")
        print(f"Created {CHAT_FILE}. You can write notes in this file to communicate with the LLM.")

def read_chat_file():
    if os.path.exists(CHAT_FILE):
        with open(CHAT_FILE, 'r') as f:
            return f.read().strip()
    return ""

def check_chat_updates():
    global last_chat_content, chat_updated
    current_content = read_chat_file()
    if current_content != last_chat_content:
        chat_updated = True
        return True
    return False

def wait_for_user_input():
    input("Press Enter to continue...")
    global chat_updated, last_chat_content
    last_chat_content = read_chat_file()
    chat_updated = False

last_inspected_files = []

HasUserInterrupted = False


def test_and_debug_mode(llm_client):
    global unchanged_files, last_inspected_files

    # Add this function to handle unexpected terminations
    def handle_unexpected_termination(signum, frame):
        print("Unexpected termination detected. Saving current state...")
        save_command_history(command_history)
        sys.exit(1)

    # Register the signal handler
    signal.signal(signal.SIGTERM, handle_unexpected_termination)

    with open('project_summary.md', 'r') as f:
        project_summary = f.read()

    technical_brief = load_technical_brief()
    directory_summaries = technical_brief.get("directory_summaries", {})
    project_structure = read_project_structure()
    test_progress = load_test_progress()
    command_history = load_command_history()
    
    print("Entering test and debug mode...")
    print(f"Completed tests: {test_progress['completed_tests']}")
    print(f"Current step: {test_progress['current_step']}")
    print(f"Command history: {json.dumps(command_history, indent=2)}")

    iteration = len(command_history) + 1



    def handle_user_suggestion():
        print("\nCtrl+C pressed. You can type 'exit' to quit or provide a suggestion.")
        user_input = input("Your input (exit/suggestion): ").strip()
        if user_input.lower() == 'exit':
            kill_all_processes()
            print("Exiting the program.")
            sys.exit(0)
        else:
            return user_input

    global HasUserInterrupted
    def handle_interrupt(signum, frame):
        global HasUserInterrupted
        if HasUserInterrupted:
            print("\nSecond Ctrl+C received. Exiting the program.")
            kill_all_processes()
            sys.exit(0)
        HasUserInterrupted = True

    # Set up the signal handler
    signal.signal(signal.SIGINT, handle_interrupt)

    retry_with_expert = False

    global unchanged_files, last_chat_content, chat_updated, chat_updated_iteration
    last_chat_content = read_chat_file()

    # Previous action analysis
    previous_action_analysis = None
    ModifiedFile = False

    while True:
        # Check for chat updates at the start of each iteration
        # check_all_processes()
        retry_with_expert = False

        if HasUserInterrupted:
            user_suggestion = handle_user_suggestion()

        if check_chat_updates():
            chat_updated_iteration = iteration
            print("Chat file updated. Pausing...")
            wait_for_user_input()

        last_10_iterations = get_last_10_iterations(command_history)

        # Collect information about running processes and their latest output
        process_status = []
        process_outputs = []
        for process_info in running_processes[:]:  # Use a copy of the list to safely modify it
            status, output = check_process_output(process_info["cmd"])
            if "has terminated" in status:
                print(status)
                continue  # Skip terminated processes
            process_status.append(status)
            if output:
                process_outputs.append(f"Latest output from '{process_info['cmd']}':\n{output[-2000:]}")

        prompt = f"""
        You are in test and debug mode for the project. You are a professional software architect and developer. Adhere to the directives, best practices and provide accurate responses based on the project context. You can refer to the project summary, technical brief, and project structure for information.

        Project Summary:
        {project_summary}

        Project Structure:
        {json.dumps(project_structure, indent=2)}

        Command History (last 10 commands):
        {json.dumps(last_10_iterations, indent=2)}

        Administrator's Notes: 
        {last_chat_content}
        
        {"Administrator suggestions for this action: " + user_suggestion if HasUserInterrupted else ""}

        {"Previous action result/analysis: " + previous_action_analysis if previous_action_analysis else ""}

        Process Status:
        {', '.join(process_status)}

        Latest Process Outputs:
        {', '.join(process_outputs) if process_outputs else "No new output from background processes."}

        {"You modified a file in the previous iteration. If you are done with code changes and moving to testing, remember to start/restart the appropriate process using INDEF/RESTART " if ModifiedFile else ""}

        Directives:
        CRITICAL: Use the command history (especially the previous command) and notes to learn from previous interactions and provide accurate responses. Avoid repeating the same actions. Additional importance to user suggestions.
        0. Follow a continuous development, integration, and testing workflow. Do This includes writing code, testing, debugging, and fixing issues.
        0. Put higher emphasis on the result/anlysis from the last iteration to make progress.
        1. When doing development, consider reading multiple files to better integrate the current file with the rest of the project.
        2. Never change code due to development environmental factors (ports, paths, etc.) unless explicitly mentioned in the prompt.
        3. If there are environment related issue, use raw commands to fix them.
        4. Use the files in the project structure to understand the context and provide accurate responses. Do not add new files.
        5. Make sure that we're making progress with each step. If we go around in circles, assume that debug is wrong and start from the beginning.
        6. Do not repeat the same action multiple times unless absolutely necessary.
        7. RESTART a process after making changes to the code. This is crucial for the changes to take effect.
         
        You can take the following actions:

        1. Run a command/test from {', '.join(ALLOWED_COMMANDS)} or {', '.join(APPROVAL_REQUIRED_COMMANDS)} syncronously (blocking), use: "RUN: {', '.join(ALLOWED_COMMANDS)}". The script will wait for the command to finish and provide you with the output.
        2. Run a command/test from {', '.join(ALLOWED_COMMANDS)} or {', '.join(APPROVAL_REQUIRED_COMMANDS)} asyncronously (non-blocking), use: "INDEF: <command>". This will run the command in the background and provide you with the initial output.
        3. Run a raw command that requires approval, use: "RAW: <raw_command>". This will run the command in the shell and provide you with the output. You can use this for any command that is not in the allowed list.
        4. Check the output current running process using "CHECK: <command>"
        5. Inspect up to four files in the project structure by replying with "INSPECT: <file_path>, <file_path>, ..."
        6. Inspect multiple files (minimum: 4, maximum: 4) and modify one (one of the files that being inspected only) by replying with "MULTI: <file_path1>, <file_path2>, <file_path3>, <file_path4>; MODIFY: <file_path(1,2,3,4)>" 
        7. Ask the user for help by replying with "HELP: <your question>". Do this when you see that no progress is being made.
        8. Restart a running process with "RESTART: <command>"
        9. Finish testing by replying with "DONE"

        Provide your response in the following format:
        ACTION: <your chosen action>
        GOALS: <Context for the goals are when your executing the command>
        REASON: <Provide this as reason and context for when you're executing the actual command (max 80 words)>

        What would you like to do next to progress towards project completion?
        """

        # if running_processes:
        #     final_prompt = prompt + prompt_prcoesses + prompt_extension
        # else:
        final_prompt = prompt # + prompt_extension

        HasUserInterrupted = False
        ModifiedFile = False
        previous_action_analysis = None

        # For debug, print the process outputs been provided
        print(f"Process outputs for debug: {process_outputs}")
        
        
        print(f"\nGenerating next step (Iteration {iteration})...")
        # Print the prompt for the user
        # print(final_prompt)
        response = llm_client.generate_response(final_prompt, 4000)
        print(f"LLM response:\n{response}")

        # Parse the response
        action_match = re.search(r'ACTION:\s*(.*)', response)
        reason_match = re.search(r'REASON:\s*(.*)', response)
        goals_match = re.search(r'GOALS:\s*((?:\d+\.\s*.*\n?)+)', response, re.DOTALL)
        notes_match = re.search(r'NOTES:\s*((?:\d+\.\s*.*\n?)+)', response, re.DOTALL)

        if action_match:
            action = action_match.group(1).strip()
            reason = reason_match.group(1).strip() if reason_match else "No reason provided"
            goals = goals_match.group(1).strip() if goals_match else "No goals provided"
            command_entry = {"iteration": iteration, "action": action, "reason": reason, "goals": goals}
            command_entry["process_outputs"] = process_outputs
            if notes_match:
                new_notes = notes_match.group(1).strip()
                command_entry["notes_updated"] = True 
                update_notes(new_notes)
                print(f"Updated notes:\n{json.dumps(llm_notes, indent=2)}")

            if action.upper().startswith("NOTES:"):
                new_notes = action.split(":", 1)[1].strip()
                update_notes(new_notes)
                print(f"Updated notes:\n{json.dumps(llm_notes, indent=2)}")
                command_entry["notes_updated"] = True

            elif action.upper().startswith("HELP:"):
                question = action.split(":")[1].strip()
                print(f"\nAsking for help with the question: {question}")
                user_response = input("Please provide your response to the model's question: ")        
                command_entry["user"] = user_response

            elif action.upper().startswith("INSPECT:"):
                file_paths = action.split(":")[1].strip()
                try:
                    inspect_files = [f.strip() for f in file_paths.split(",")]
                    file_contents = {}

                    # Check if the current set of files is the same as the last inspected set
                    if set(inspect_files) == set(last_inspected_files):
                        error_msg = "Error: Cannot inspect the same set of files consecutively. Please include at least one different file."
                        print(error_msg)
                        command_entry["error"] = error_msg
                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue

                    # Update the last_inspected_files
                    last_inspected_files = inspect_files

                    for file_path in inspect_files:
                        if not os.path.exists(file_path):
                            error_msg = f"Error: File not found: {file_path}"
                            print(error_msg)
                            file_contents[file_path] = error_msg
                        else:
                            file_contents[file_path] = read_file(file_path)

                    inspection_prompt = f"""
                    {prompt}

                    You chose to inspect the following files: {', '.join(inspect_files)}

                    Reason for this action: {reason}

                    Goals for this action: {goals}

                    Inspect for dependencies between the files. Check that variables, functions, parameters, and return values are used correctly and consistently across the files.

                    Inspected files:
                    """

                    for file_path, content in file_contents.items():
                        inspection_prompt += f"""
                        File: {file_path}
                        Content:
                        {content}

                        """

                    inspection_prompt += """
                    Respond to yourself in 100 words or less with the results of the inspection. This is for the result section of this command, provide specific instructions to yourself for the next step such as specific changes in the code. If no improvements are needed, state that the files are ready for testing, or provide debug notes:
                    """

                    analysis = llm_client.generate_response(inspection_prompt, 4000)
                    previous_action_analysis = analysis
                    print(f"Files analysis:\n{analysis}")

                    # for file_path, content in file_contents.items():
                    #     if "Error: File not found" not in content:
                    #         technical_brief = update_technical_brief(file_path, content, iteration, mode="test", test_info=analysis)

                    update_test_progress(current_step=f"Inspected files: {', '.join(inspect_files)}")
                    command_entry["result"] = {"analysis": analysis}

                except Exception as e:
                    error_msg = f"Error inspecting files: {str(e)}"
                    print(error_msg)
                    command_entry["error"] = error_msg
                    wait_for_user()

            elif action.upper().startswith("REWRITE:"):
                file_path = action.split(":")[1].strip()
                if not os.path.exists(file_path):
                    error_msg = f"Error: File not found: {file_path}\n You cannot create a new file. Try to implement the functionality in an existing file in the project structure or ask user for help."
                    command_entry["error"] = error_msg
                    print("File not found. Provided LLM with the error message and suggestion.")
                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue
                current_content = read_file(file_path)
                file_brief = get_file_technical_brief(technical_brief, file_path)
                modification_prompt = f"""
                {prompt}

                You requested to inspect and rewrite the file {file_path}.

                File content:
                {current_content}

                Reason for this action: {reason}

                Goals for this action: {goals}

                Please provide the updated content for this file, addressing any issues or improvements needed based on your reason. Your output should be valid code ONLY, without any explanations or comments outside the code itself. If you need to include any explanations, please do so as comments within the code.
                """
                if retry_with_expert:
                    new_content = llm_client.generate_response(modification_prompt, 4096)
                else:
                    new_content = llm_client.generate_response(modification_prompt, 8192)

                extracted_content = extract_content(new_content, file_path)
                
                modify_file(file_path, extracted_content)
                print(f"\nModified {file_path}")
                
                changes_prompt = f"""
                You are a professional software architect and developer.

                You inspected and rewrote one file.

                Reason given for this action: {reason}

                Goals given for this action: {goals}

                Command history (last 10 commands) for better context: {json.dumps(last_10_iterations, indent=2)}

                Summarize the changes made to the file {file_path}. Compare the original content:
                {current_content}

                With the new content:
                {extracted_content}

                This is for the result section of this command. Provide a brief summary of the modifications in 50 words or less and if the goals were achieved.
                """
                changes_summary = llm_client.generate_response(changes_prompt, 1000)
                print(f"Changes summary:\n{changes_summary}")
                
                technical_brief = update_technical_brief(file_path, extracted_content, iteration, mode="test", test_info=changes_summary)
                update_test_progress(current_step=f"Modified {file_path}")
                
                command_entry["result"] = {"changes_summary": changes_summary}

                # If change summary has "FILES ARE IDENTICAL", set retry_with_expert to True
                # if "FILES ARE IDENTICAL" in changes_summary:
                #     retry_with_expert = True
                #     # Change model to expert
                #     if isinstance(llm_client, VertexAILLM):
                #         llm_client.switch_model("claude-3-opus@20240229")
                #     continue

            elif action.upper().startswith("MULTI:"):
                parts = action.split(";")
                inspect_files = [f.strip() for f in parts[0].split(":")[1].split(",")]
                write_file = parts[1].split(":")[1].strip()

                # Check if the file is in the unchanged_files list and still under constraint
                if write_file in unchanged_files and unchanged_files[write_file] > 0:
                    error_msg = f"Error: The file {write_file} cannot be modified for {unchanged_files[write_file]} more iterations due to no changes in the previous attempt."
                    command_entry["error"] = error_msg
                    print(error_msg)
                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue

                if write_file not in inspect_files:
                    error_msg = f"Error: The file to be written ({write_file}) must be one of the inspected files."
                    # Check if error field is already present in the command entry, then concat less set
                    if "error" not in command_entry:
                        command_entry["error"] = error_msg
                    else:
                        command_entry["error"] += error_msg
                    print(error_msg)
                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue

                file_contents = {}
                for file_path in inspect_files:
                    if not os.path.exists(file_path):
                        error_msg = f"Error: File not found: {file_path}"
                        file_contents[file_path] = error_msg
                        if "error" not in command_entry:
                            command_entry["error"] = error_msg
                        else:
                            command_entry["error"] += error_msg
                    else:
                        content = read_file(file_path)
                        # numbered_content = add_line_numbers(content)
                        file_contents[file_path] = content

                if write_file not in file_contents or file_contents[write_file].startswith("Error: File not found"):
                    print(f"The file to be written ({write_file}) does not exist.")
                    user_approval = input(f"Do you want to create the file {write_file}? (yes/no): ").lower().strip()
                    if user_approval == 'yes':
                        # Create the directory if it doesn't exist
                        os.makedirs(os.path.dirname(write_file), exist_ok=True)
                        # Create an empty file
                        open(write_file, 'w').close()
                        print(f"Created new file: {write_file}")
                        file_contents[write_file] = ""  # Add empty content to file_contents

                        # Update project structure
                        update_project_structure(write_file)
                        print(f"Updated project structure with new file: {write_file}")
                    else:
                        error_msg = f"User denied creation of new file: {write_file}. You should work with existing files only."
                        command_entry["error"] = error_msg
                        print(error_msg)
                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue

                inspection_prompt = f"""
                {prompt}

                You are a professional software architect and developer.

                You chose to inspect multiple files and modify one of them.

                Inspected files:
                """

                for file_path, content in file_contents.items():
                    inspection_prompt += f"""
                File: {file_path}
                Content:
                {content}

                """

                inspection_prompt += f"""
                You will modify the file addressing the following goals.

                {"Previous action result/analysis: " + previous_action_analysis if previous_action_analysis else ""}

                Reason for this action: {reason}

                Goals for this action: {goals}

                Please provide the complete updated content for the file {write_file}, addressing any issues or improvements needed based on your inspection of all the files, while keeping code CONSISTENT across files, you must not make an unnecessary changes to the code. Never remove features unless specified. You must provide the full content since this is directly written to the file without processing. Your output should be valid code ONLY, without any explanations or comments outside the code itself. If you need to include any explanations, please do so as comments within the code.
                """
                # Use the following format to provide changes for the file. There should be no other content in your response, only changes to the file content:
                # - To add a line after a line number: +<line_number>:new_content
                # - To remove a line: -<line_number>
                # - To modify a line: <line_number>:new_content

                # If retry_with_expert is set, token = 4096, else 8192
                if retry_with_expert:
                    new_content = llm_client.generate_response(inspection_prompt, 4096)
                else:
                    new_content = llm_client.generate_response(inspection_prompt, 8192)

                # new_content = llm_client.generate_response(inspection_prompt, )  # Increased token limit for multiple files
                extracted_content = extract_content(new_content, write_file)

                # Print changes
                # print(f"Changes:\n{changes}")
                # Parse and apply changes
                # current_content = remove_line_numbers(file_contents[write_file])
                # parsed_changes = parse_changes(changes)
                # new_content = apply_changes(current_content, parsed_changes)
                                
                changes_prompt = f"""
                You are a professional software architect and developer.

                You inspected multiple files and modified one of them. 

                Reason given for this action: {reason}

                Goals given for this action: {goals}

                Command history (last 10 commands) for better context: {json.dumps(last_10_iterations, indent=2)}

                Summarize the changes made to the file {write_file} for future notes to yourself. Compare the original content:
                {read_file(write_file)}

                With the new content:
                {extracted_content}

                This is for the result section of this command. Provide a brief summary of the modifications and if the goals were achieved in 100 words or less:
                """
                changes_summary = llm_client.generate_response(changes_prompt, 1000)
                previous_action_analysis = changes_summary
                print(f"Changes summary:\n{changes_summary}")

                changes_made = compare_and_write(write_file, extracted_content)
                if not changes_made:
                    print("Warning: No actual changes were made in this iteration.")
                    command_entry["result"] = {"warning": "No actual changes were made in this iteration."}

                    # Add the file to unchanged_files with a counter of 2
                    unchanged_files[write_file] = 2

                    command_entry["error"] = f"The file {write_file} cannot be modified for the next 2 successful iterations due to no changes in this attempt."

                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue

                modify_file(write_file, extracted_content)
                print(f"\nModified {write_file}")
                ModifiedFile = True
                
                technical_brief = update_technical_brief(write_file, new_content, iteration, mode="test", test_info=changes_summary)
                update_test_progress(current_step=f"Inspected multiple files and modified {write_file}")
                
                command_entry["result"] = {"changes_summary": changes_summary}

                # If change summary has "FILES ARE IDENTICAL", set retry_with_expert to True
                # if "FILES ARE IDENTICAL" in changes_summary:
                #     retry_with_expert = True
                #     # Change model to expert
                #     if isinstance(llm_client, VertexAILLM):
                #         llm_client.switch_model("claude-3-opus@20240229")
                #     continue

            elif action.upper() == "DONE":
                print("\nTest and debug mode completed.")
                command_entry["result"] = "Test and debug mode completed."
                break

            # Handle raw commands
            elif action.upper().startswith("RAW:"):
                print(f"\nExecuting raw command: {action}")
                output, success = execute_command(action)
                print(f"Command output:\n{output}")
                update_test_progress(completed_test=action, current_step=f"Executed raw command: {action}")

                command_entry["output"] = output[:12000] if len(output) < 12000 else "Output truncated due to length"
                previous_action_analysis = output
                command_entry["success"] = success

            elif action.upper().startswith("INDEF:"):
                print(f"\nExecuting command: {action}")
                output, success = execute_command(action)
                print(f"Command output:\n{output}")
                update_test_progress(completed_test=action, current_step=f"Executed {action}")

                command_entry["output"] = output[:12000] if len(output) < 12000 else "Output truncated due to length"
                command_entry["success"] = success

                previous_action_analysis = output

            # Analysis step for CHECK commands
            elif action.upper().startswith("CHECK:"):
                print(f"\nChecking: {action}")
                output, success = execute_command(action)
                analysis_prompt = f"""
                {prompt}

                You requested to check this command: {action}.

                You gave this reason: {reason}

                You set these goals: {goals}

                Check result:
                {output}

                This is for the result section of this command. Analyze the check result and determine if further action is needed. Respond in 100 words or less:
                """
                analysis = llm_client.generate_response(analysis_prompt, 1000)

                previous_action_analysis = analysis
                print(f"Check analysis:\n{analysis}")
                command_entry["analysis"] = analysis
            
            elif action.upper().startswith("RESTART:"):
                cmd = action.split(":", 1)[1].strip()
                output = restart_process(cmd)
                print(output)
                command_entry["result"] = {"restart_output": output}

            elif action.upper().startswith("RUN:"):
                action = action[4:].strip()            
                if any(action.startswith(cmd) for cmd in ALLOWED_COMMANDS + APPROVAL_REQUIRED_COMMANDS):
                    env_check, env_output = check_environment(action)
                    if env_check:
                        print(f"\nExecuting command: {action}")
                        output, success = execute_command(action)
                        print(f"Command output:\n{output}")
                        update_test_progress(completed_test=action, current_step=f"Executed {action}")

                        if "This command appears to start a long-running process" in output:
                            command_entry["suggestion"] = output
                        else:                      
                            analysis_prompt = f"""
                            {prompt}

                            You requested to run this command: {action}.

                            You gave this reason: {reason}

                            You set these goals: {goals}

                            Output:
                            {output}

                            Execution {'succeeded' if success else 'failed'}

                            This is for the result section of this command. Respond based on the command execution in 200 words or less (lesser the better). Provide specifics on the success of the command, any errors encountered, and the next steps based on the output. If a test was run, provide the results and any debugging steps (with errors in specific files and lines) trying to fix issues one by one:
                            """
                            analysis = llm_client.generate_response(analysis_prompt, 1000)
                            print(f"Command analysis:\n{analysis}")
                            # Only add the command output if the length is less than 250 characters
                            if len(output) < 12000:
                                command_entry["output"] = output
                            command_entry["success"] = success
                            command_entry["analysis"] = analysis
                    else:
                        error_msg = f"Environment check failed. Cannot execute command: {action}"
                        print(error_msg)
                        command_entry["error"] = f"{env_output}"
                        command_entry["result"] = {"error": error_msg, "env_output": env_output}
                        wait_for_user()

            else:
                error_msg = f"Error: Invalid action: {action}"
                print(error_msg)
                command_entry["error"] = error_msg
            
            command_history.append(command_entry)
            save_command_history(command_history)
        else:
            print("Invalid response format. Please provide an action.")
            command_entry["error"] = "Invalid response format. Please provide an action."

        test_progress = load_test_progress()
        iteration += 1
        save_command_history(command_history)

        # Decrease the counter for unchanged files at the end of each iteration
        for file in list(unchanged_files.keys()):
            unchanged_files[file] -= 1
            if unchanged_files[file] == 0:
                del unchanged_files[file]

        if retry_with_expert:
            # Switch back
            if isinstance(llm_client, VertexAILLM):
                llm_client.switch_model("claude-3-5-sonnet-20240620")
            retry_with_expert = False

    kill_all_processes()

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
    
    iterations = [file.get("last_updated_iteration", 0) for file in all_files]
    current_iteration = min(iterations) if iterations else 0
    max_iterations = 2
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
    # Ensure chat file exists before entering any mode
    ensure_chat_file_exists()

        # Check if project_structure.json exists
    project_structure = read_project_structure()
    if project_structure is None:
        user_input = input("project_structure.json not found. Do you want to generate it? (yes/no): ").lower()
        if user_input == 'yes':
            project_structure = generate_project_structure()
            save_project_structure(project_structure)
            print("project_structure.json has been generated.")
        else:
            print("Please create a project_structure.json file manually before proceeding.")
            return

    mode = input("Enter mode (generate/test): ").lower()
    
    if mode == "test":
        test_and_debug_mode(llm_client)
    elif mode == "generate":
        generate()
    else:
        print("Invalid mode. Please choose 'generate' or 'test'.")
        return
    
if __name__ == "__main__":
    try:
        main()
    finally:
        kill_all_processes()
else:
    # For testing purposes
    __all__ = ['parse_changes', 'apply_changes']