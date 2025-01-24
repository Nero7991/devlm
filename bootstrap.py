# MIT License
# 
# Copyright (c) 2024 Oren Collaco
# 
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
# 
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
# 
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

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
import argparse
import queue
import psutil
from typing import List, Dict
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager

# Global variables for model and source settings
MODEL = 'claude'  # Default to 'claude'
SOURCE = 'anthropic'  # Default to 'anthropic'
API_KEY = None
PROJECT_ID = None
REGION = None
OPENAI_BASE_URL = 'https://api.openai.com/v1'
NEWLINE = "\n"
DEBUG_PROMPT = False

# Define the devlm folder path
DEVLM_FOLDER = ".devlm"
GLOBAL_MAX_PROMPT_LENGTH = 200000
Global_error = ""
GLOBAL_ERROR_PROMPT_LENGTH = "Prompt length is too long. Truncated to 200000 characters. However, this is a FATAL problem that will prevent the LLM from getting other relevant information making it useless. Figure out what is causing prompt length to be too long and fix it."

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
    'g++',
    'gcc',
    'make',
    'ls',
    'mkdir',
    'cp',
    'mv',
    'chmod',
    'chown',
    'lsof',
    'netstat',
    'ss',
    'pgrep',
    'erlc',
    'echo',
    'erl',
    'west build',
    'git clone',
    # Add more commands as needed
]

APPROVAL_REQUIRED_COMMANDS = [
    'sudo apt install',
    './',
    # Add a raw command that requires approval
    'RAW: <raw_command>'
]

try:
    import anthropic
    from anthropic import AnthropicVertex
except ImportError:
    print("Error: anthropic package is not installed. Please run: pip install anthropic[vertex]")
    sys.exit(1)

class LLMError(Exception):
    def __init__(self, error_type, message):
        self.error_type = error_type
        self.message = message
        super().__init__(f"{error_type}: {message}")

class LLMInterface(abc.ABC):
    @abc.abstractmethod
    def generate_response(self, prompt: str, max_tokens: int) -> str:
        pass

    def _write_debug_prompt(self, prompt: str):
        global DEBUG_PROMPT
        if DEBUG_PROMPT:
            with open(os.path.join(DEBUG_PROMPT_FOLDER, f"prompt_{datetime.now().strftime('%Y%m%d_%H%M%S')}.txt"), "w") as f:
                f.write(prompt)

    def _write_debug_response(self, response: str):
        global DEBUG_PROMPT
        if DEBUG_PROMPT:
            os.makedirs(DEBUG_RESPONSE_FOLDER, exist_ok=True)
            with open(os.path.join(DEBUG_RESPONSE_FOLDER, f"response_{datetime.now().strftime('%Y%m%d_%H%M%S')}.txt"), "w") as f:
                f.write(response)

class AnthropicLLM(LLMInterface):
    def __init__(self, client):
        self.client = client

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        self._write_debug_prompt(prompt)
        Global_error = ""
        # make sure the prompt length is less than 200000 else truncate it
        if len(prompt) > GLOBAL_MAX_PROMPT_LENGTH:
            prompt = prompt[:GLOBAL_MAX_PROMPT_LENGTH]
            Global_error = GLOBAL_ERROR_PROMPT_LENGTH
            print(Global_error)
        while True:
            try:
                response = self.client.messages.create(
                    model="claude-3-5-sonnet-20241022",
                    max_tokens=max_tokens,
                    messages=[
                        {"role": "user", "content": prompt}
                    ]
                )
                self._write_debug_response(response.content[0].text if response.content else "")
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
            if self.retries < self.max_retries:
                wait_time = self._calculate_wait_time(self.retries)
                print(f"Internal server error. Retrying in {wait_time} seconds...")
                time.sleep(wait_time)
                self.retries += 1
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

class OpenAILLM(LLMInterface):
    def __init__(self, api_key: str, model: str = "gpt-4", base_url: Optional[str] = None):
        # Import required modules only when OpenAI LLM is initialized
        try:
            import openai
            from openai import OpenAI
            import time
            import random
            import os
            import re
            from typing import Optional
        except ImportError as e:
            missing_package = str(e).split("'")[1]
            if missing_package == "openai":
                print("Error: openai package is not installed. Please run: pip install openai")
            else:
                print(f"Error: Required package {missing_package} is not installed.")
            sys.exit(1)

        # Store necessary imports as class attributes to use in other methods
        self._openai = openai
        self._OpenAI = OpenAI
        self._time = time
        self._random = random
        self._re = re
        
        # Initialize the client with optional base_url
        client_kwargs = {"api_key": api_key}
        if base_url:
            client_kwargs["base_url"] = base_url
            print(f"Using custom OpenAI API server: {base_url}")
        
        try:
            self.client = self._OpenAI(**client_kwargs)
        except Exception as e:
            print(f"Error initializing OpenAI client: {str(e)}")
            raise

        self.model = model
        self.max_retries = 5
        self.base_delay = 1
        self.retries = 0
        self.base_url = base_url

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        self._write_debug_prompt(prompt)
        # Make sure the prompt length is less than 200000 else truncate it
        if len(prompt) > GLOBAL_MAX_PROMPT_LENGTH:
            prompt = prompt[:GLOBAL_MAX_PROMPT_LENGTH]
            Global_error = GLOBAL_ERROR_PROMPT_LENGTH
            print(Global_error)

        while True:
            try:
                #print the message length
                print(f"Message length: {len(prompt)}")
                response = self.client.chat.completions.create(
                    model=self.model,
                    messages=[
                        {"role": "user", "content": prompt}
                    ],
                    max_tokens=max_tokens
                )
                self._write_debug_response(response.choices[0].message.content if response.choices else "")
                return response.choices[0].message.content if response.choices else ""

            except self._openai.RateLimitError as e:
                if self._handle_error("rate_limit_error", str(e)):
                    continue
                raise LLMError("rate_limit_error", str(e))

            except self._openai.APIError as e:
                if self._handle_error("api_error", str(e)):
                    continue
                raise LLMError("api_error", str(e))

            except self._openai.APIConnectionError as e:
                if self._handle_error("connection_error", str(e)):
                    continue
                raise LLMError("connection_error", str(e))

            except self._openai.InsufficientQuotaError as e:
                if self._handle_error("insufficient_quota", str(e)):
                    continue
                raise LLMError("insufficient_quota", str(e))

            except self._openai.InvalidRequestError as e:
                raise LLMError("invalid_request", str(e))

            except Exception as e:
                print(f"Unexpected error: {str(e)}")
                raise

    def _handle_error(self, error_type: str, error_message: str) -> bool:
        print(f"Error type: {error_type}")
        print(f"Error message: {error_message}")
        if error_type == "rate_limit_error":
            wait_time = self._extract_wait_time(error_message)
            print(f"Rate limit exceeded. Retrying in {wait_time} seconds...")
            self._time.sleep(wait_time)
            return True

        elif error_type == "api_error":
            if self.retries < self.max_retries:
                wait_time = self._calculate_wait_time(self.retries)
                print(f"API error. Retrying in {wait_time} seconds...")  
                self._time.sleep(wait_time)
                self.retries += 1
                return True
            return False

        elif error_type == "connection_error":
            if self.retries < self.max_retries:
                wait_time = self._calculate_wait_time(self.retries)
                print(f"Connection error. Retrying in {wait_time} seconds...")
                self._time.sleep(wait_time)
                self.retries += 1
                return True
            return False

        elif error_type == "insufficient_quota":
            print("Insufficient quota. Please check your OpenAI account balance.")
            input("Press Enter once you've added credit to continue, or Ctrl+C to exit...")
            return True

        return False

    def _calculate_wait_time(self, retries: int) -> float:
        """Calculate wait time using exponential backoff with jitter."""
        return self.base_delay * (2 ** retries) + self._random.uniform(0, 1)

    def _extract_wait_time(self, error_message: str) -> int:
        """Extract wait time from rate limit error message."""
        try:
            match = self._re.search(r'(\d+)\s*seconds?', error_message.lower())
            if match:
                return int(match.group(1))
        except:
            pass
        return 60  # Default wait time if we can't extract it from the message

    def switch_model(self, new_model: str):
        """Switch to a different OpenAI model."""
        self.model = new_model
        print(f"Switched to model: {self.model}")

    def get_server_info(self) -> dict:
        """Get information about the current server configuration."""
        return {
            "base_url": self.base_url or "https://api.openai.com/v1",
            "model": self.model,
            "max_retries": self.max_retries
        }

class VertexAILLM(LLMInterface):
    def __init__(self, project_id: str, region: str, model: Optional[str] = None):
        self.project_id = project_id
        self.region = region
        self.client = AnthropicVertex(region=region, project_id=project_id)
        self.max_retries = 5
        self.retry_delay = 32  # Start with 32 seconds delay
        self.model = model or "claude-3-5-sonnet-v2@20241022"  # Default model

    def generate_response(self, prompt: str, max_tokens: int) -> str:
        self._write_debug_prompt(prompt)
        # make sure the prompt length is less than 200000 else truncate it
        if len(prompt) > 200000:
            prompt = prompt[:200000]
            Global_error = GLOBAL_ERROR_PROMPT_LENGTH
            print(Global_error)
        messages = [{"role": "user", "content": prompt}]
        full_response = ""
        iteration = 0
        max_iterations = 4  # Limit the number of iterations to prevent infinite loops

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
                            "Your previous output starts after the third greater than sign in <<<START>>> and ends at the character before the first less than sign in <<<END>>>. Please continue the output (adding new line and tabs if needed at the beginning), picking up where you left off without repeating information, your output will be appended without modification before first less than sign in <<<END>>>. Do not include anything other than the continuation of the output."
                        )
                        messages = [{"role": "user", "content": continuation_prompt}]
                        iteration += 1
                        break
                    else:
                        self._write_debug_response(full_response)
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
        self._write_debug_response(full_response)
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

        # Replace with your actual Google Cloud project ID and region
        #project_id = "devlm-435701"
        project_id = PROJECT_ID
        region = REGION
        return VertexAILLM(project_id, region, model)
    elif provider == "openai":
        import os
        if not API_KEY:
            api_key = os.getenv(API_KEY)
            if not api_key:
                raise ValueError("OpenAI API key not provided and API_KEY environment variable not set")
        return OpenAILLM(
            api_key=API_KEY, 
            model=model or "gpt-4o",
            base_url=SERVER
        )
    else:
        raise ValueError(f"Unsupported LLM provider: {provider}")

# Update the global llm_client variable
llm_client = None

# llm_client = get_llm_client()

# Initialize the Anthropic client
# client = anthropic.Anthropic(api_key=API_KEY)

TECHNICAL_BRIEF_FILE = os.path.join(DEVLM_FOLDER, "project_technical_brief.json")
TEST_PROGRESS_FILE = os.path.join(DEVLM_FOLDER, "test_progress.json")
CHAT_FILE = os.path.join(DEVLM_FOLDER, "chat.txt")
PROJECT_STRUCTURE_FILE = os.path.join(DEVLM_FOLDER, "project_structure.json")
DEBUG_PROMPT_FOLDER = os.path.join(DEVLM_FOLDER + "/debug/prompts/")
DEBUG_RESPONSE_FOLDER = os.path.join(DEVLM_FOLDER + "/debug/responses/")
TASK = None
WRITE_MODE = 'diff'
MAX_FILE_LENGTH = 20000

# Update the COMMAND_HISTORY_FILE and HISTORY_BRIEF_FILE
COMMAND_HISTORY_FILE = os.path.join(DEVLM_FOLDER+ "/actions", f"action_history_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json")
HISTORY_BRIEF_FILE = os.path.join(DEVLM_FOLDER+ "/briefs", f"history_brief_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json")

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
    structure_file = os.path.join(DEVLM_FOLDER, "project_structure.json")
    if os.path.exists(structure_file):
        with open(structure_file, "r") as f:
            return json.load(f)
    else:
        return {
            "": []
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
                if not os.path.exists(file_path):
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
            if item == 'node_modules' or item.startswith('.') or item == 'build':
                continue
            full_path = os.path.join(path, item)
            if os.path.isfile(full_path):
                structure[""].append(item)
            elif os.path.isdir(full_path):
                structure[item] = create_structure(full_path)
        return structure

    return create_structure(root_dir)

def save_project_structure(structure):
    with open(PROJECT_STRUCTURE_FILE, 'w') as f:
        json.dump(structure, f, indent=2)

def read_project_structure():
    if os.path.exists(PROJECT_STRUCTURE_FILE):
        with open(PROJECT_STRUCTURE_FILE, 'r') as f:
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


def require_approval(command):
    print(f"The following command requires your approval:")
    print(command)
    approval = input("Do you approve this command? (yes/no): ").lower().strip()
    return approval == 'yes'

def wait_for_user():
    print(f"Some error occurred. Please resolve the issue and press Enter to continue.")
    input()
    return True

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

def get_all_child_processes(parent_pid):
    try:
        parent = psutil.Process(parent_pid)
        children = parent.children(recursive=True)
        return children
    except psutil.NoSuchProcess:
        return []

def check_and_terminate_existing_process(command):
    global running_processes
    
    for process_info in running_processes:
        if process_info['cmd'] == command:
            pids_to_terminate = process_info['child_pids'] + [process_info['pid']]
            for pid in pids_to_terminate:
                try:
                    process = psutil.Process(pid)
                    print(f"Terminating process with PID: {pid}")
                    process.terminate()
                    
                    try:
                        process.wait(timeout=5)
                    except psutil.TimeoutExpired:
                        print(f"Process {pid} did not terminate within timeout. Forcing termination.")
                        process.kill()
                    
                    print(f"Process {pid} has been terminated.")
                except psutil.NoSuchProcess:
                    print(f"Process {pid} no longer exists.")
                except psutil.AccessDenied:
                    print(f"Access denied when trying to terminate process {pid}.")
            
            # Remove the terminated process from the list
            running_processes = [p for p in running_processes if p['pid'] != process_info['pid']]
            time.sleep(2)  # Wait for 2 seconds to ensure resources are released
            return True
    
    return False

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
    # For npm commands, use the script name as the key
    if run_part.startswith('npm run'):
        return run_part.split()[-1]
    return run_part

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
        
        # Wait for the process to start and get all child processes
        time.sleep(5)
        child_processes = get_all_child_processes(process.pid)
        child_pids = [child.pid for child in child_processes]
        
        # Add the new process to the list
        process_info = {
            "cmd": command,
            "process": process,
            "queue": output_queue,
            "cwd": cwd,
            "run_command": run_command,
            "pid": process.pid,
            "child_pids": child_pids
        }
        running_processes.append(process_info)
        
        # Print the process info
        print(f"Process Info: {process_info}")
        
        # Collect initial output
        initial_output = ""
        while not output_queue.empty():
            initial_output += output_queue.get_nowait()
        
        # Keep only the last 2000 characters of the initial output
        initial_output = initial_output[-2000:]
        
        return f"Started new process: {command}\nInitial PID: {process.pid}\nChild PIDs: {child_pids}\nInitial output:\n{initial_output}"
    except PermissionError as e:
        error_output = f"PermissionError: {str(e)}\n"
        if run_command.endswith('.go'):
            suggestion = (
                "It seems you're trying to execute a Go file from the wrong directory. "
                "To run a Go file, use the following format:\n"
                "cd /path/to/directory && go run filename.go\n"
                "For example: cd devlm-identity && go run cmd/api/main.go"
            )
            error_output += f"\nSuggestion: {suggestion}"
        return error_output
    except Exception as e:
        return f"Error executing command: {str(e)}"
    finally:
        # Change back to the original directory
        os.chdir(cwd)

def check_process_output(command):
    process_key = get_process_key(command)
    for process_info in running_processes[:]:  # Iterate over a copy of the list
        if process_key in process_info["cmd"]:
            process = process_info["process"]
            output_queue = process_info["queue"]
            if process.poll() is not None:
                running_processes.remove(process_info)
                return "", ""  # Process has terminated
            output = ""
            while not output_queue.empty():
                try:
                    output += output_queue.get_nowait()
                except queue.Empty:
                    break
            # Keep only the last 2000 characters
            output = output[-3000:]
            return command, output  # Process is running
    return "", ""  # No running process found

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
    global command_decisions, frontend_testing_enabled, current_url

    if frontend_testing_enabled:
        if command.upper().startswith("UI_OPEN:"):
            url = command.split(":", 1)[1].strip()
            return ui_open_url(url), True
        elif command.upper().startswith("UI_CLICK:"):
            button_id = command.split(":", 1)[1].strip()
            return ui_click_button(button_id), True
        elif command.upper().startswith("UI_CHECK_TEXT:"):
            parts = command.split(":", 2)
            element_id = parts[1].strip()
            expected_text = parts[2].strip()
            return ui_check_element_text(element_id, expected_text), True
        elif command.upper().startswith("UI_CHECK_LOG:"):
            expected_log = command.split(":", 1)[1].strip()
            return ui_check_console_logs(expected_log), True

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
    ## Split the command into parts
    #command_parts = command.split('&&')
    #
    ## Initialize variables
    #current_dir = os.getcwd()
    #output = ""
    #return_code = 0
    #
    #try:
    #    for part in command_parts:
    #        part = part.strip()
    #        if part.startswith('cd '):
    #            # Change directory
    #            new_dir = part.split(None, 1)[1]
    #            try:
    #                os.chdir(new_dir)
    #                output += f"Changed directory to {new_dir}\n"
    #            except FileNotFoundError:
    #                output += f"Error: Directory '{new_dir}' not found\n"
    #                return_code = 1
    #                break
    #        else:
    #            # Execute the command
    #            try:
    #                process = subprocess.Popen(shlex.split(part), stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True, universal_newlines=True)
    #                stdout, stderr = process.communicate(timeout=timeout)
    #                return_code = process.returncode
    #                output += f"Command: {part}\n"
    #                output += f"STDOUT:\n{stdout}\n"
    #                output += f"STDERR:\n{stderr}\n"
    #                if return_code != 0:
    #                    output += f"Command failed with return code {return_code}\n"
    #                    break
    #            except subprocess.TimeoutExpired:
    #                process.kill()
    #                output += f"Command execution timed out after {timeout} seconds.\n"
    #                return_code = -1
    #                break
    #            except FileNotFoundError:
    #                output += f"Error: Command '{part.split()[0]}' not found\n"
    #                return_code = 1
    #                break
    current_dir = os.getcwd()
    output = ""
    return_code = 0

    try:
        # Execute the entire command as a single shell command
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True, universal_newlines=True, executable='/bin/bash')
        
        def timeout_handler(signum, frame):
            process.kill()
            raise subprocess.TimeoutExpired(command, timeout)

        # Set up the timeout
        signal.signal(signal.SIGALRM, timeout_handler)
        signal.alarm(timeout)

        try:
            stdout, stderr = process.communicate()
            return_code = process.returncode
            output += f"Command: {command}\n"
            output += f"STDOUT:\n{stdout}\n"
            output += f"STDERR:\n{stderr}\n"
            if return_code != 0:
                output += f"Command failed with return code {return_code}\n"
        except subprocess.TimeoutExpired:
            output += f"Command execution timed out after {timeout} seconds.\n"
            return_code = -1
        finally:
            signal.alarm(0)  # Cancel the alarm
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

    # only check environment if it's go or python
    if command.split('&&')[-1].strip().split()[0] not in ["go", "python", "python3"]:
        print("Skipping environment check for non-Go or non-Python command.")
        return True, "success"
    
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

def get_last_n_iterations(command_history, count):
    return command_history[-count:] if len(command_history) > count else command_history

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

def truncate_content(numbered_content, max_length):
    """
    Truncates the numbered content at the first newline after max_length characters
    and adds a truncation message.
    
    Args:
    numbered_content (str): The content with line numbers
    max_length (int): Maximum length before truncation
    
    Returns:
    str: Truncated content with truncation message
    """
    if len(numbered_content) <= max_length:
        return numbered_content
        
    # Find the first newline after max_length
    truncation_point = numbered_content.find('\n', max_length)
    if truncation_point == -1:
        return numbered_content
        
    # Get the truncated content
    truncated_content = numbered_content[:truncation_point]
    
    # Count the remaining lines and characters
    remaining_content = numbered_content[truncation_point:]
    remaining_lines = remaining_content.count('\n')
    remaining_chars = len(remaining_content.replace('\n', ''))
    
    # Add truncation message
    truncation_msg = f"\n<TRUNCATED {remaining_lines} lines and {remaining_chars} characters>"
    
    return truncated_content + truncation_msg

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
                diff = ''.join(diff)
                print(diff)
                return True, diff
            else:
                print(f"No actual changes to write in {file_path}")
                return False, None
        else:
            print(f"File {file_path} content is identical. No changes made.")
            return False, None
    except FileNotFoundError:
        return False, None

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
    
unchanged_files = {}
last_chat_content = ""
chat_updated = False
chat_updated_iteration = 0

def ensure_chat_file_exists():
    # Create the chat file if it doesn't exist
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

MAX_BRIEF_COMMANDS = 20
UPDATE_INTERVAL = 10

def load_history_brief() -> Dict:
    try:
        # make sure the file exist else create it
        if not os.path.exists(HISTORY_BRIEF_FILE):
            with open(HISTORY_BRIEF_FILE, 'w') as f:
                json.dump({}, f, indent=2)
        with open(HISTORY_BRIEF_FILE, 'r') as f:
            return json.load(f)
    except FileNotFoundError:
        return { "key_events": []}

def save_history_brief(brief: Dict):
    with open(HISTORY_BRIEF_FILE, 'w') as f:
        json.dump(brief, f, indent=2)

def update_history_brief(command_history: List[Dict], current_brief: Dict, user_goal: str, chat_content: str, project_structure: Dict) -> Dict:
    recent_commands = command_history[-30:]  # Get the last MAX_BRIEF_COMMANDS commands
    
    update_prompt = f"""
    You are an assistant tasked with maintaining a concise history brief of a software development project. Since you are only provided the last 15 raw commands, you need to extract key events and summarize the project's progress based on the command history, user messages and the previous brief. This will help in tracking the project's development and identifying any issues or challenges and prevent repetition of the same mistakes and work. Be specific and concise in your output so that the project's progress can be easily tracked.

    Recent command history (last 30 commands):
    {json.dumps(recent_commands, indent=2)}

    Please update the history brief with the following guidelines:
    1. In context of the user chat content, extract key events and summarize the project's progress.
    2. Keep crucial events from the current brief so that the history is maintained and actions are repeated.

    Respond with a JSON object in the following format without any other text:
    {{
        "key_events": [
            "User requested to add a new feature to the project.",
            "Issue X was resolved by modifying file Y.",
            "Feature A was implemented by adding a new function in file B.",
            "Issue Y was identified during testing in file Z and needs to be fixed.",
            ...
        ]
    }}
    """
    #print (update_prompt)

    response = llm_client.generate_response(update_prompt, 4000)
    print(f"History brief response: {response}")
    try:
        updated_brief = json.loads(response)
        return updated_brief
    except json.JSONDecodeError:
        print("Error: Failed to parse LLM response as JSON. Using previous brief.")
        return current_brief

def get_history_brief_for_prompt(brief: Dict) -> str:
    if not brief.get('key_events'):
        return "No key events recorded yet."
    return f"""
    Key Events:
    {chr(10).join(f"- {event}" for event in brief['key_events'])}
    """

last_inspected_files = []

HasUserInterrupted = False

# Add new global variables for frontend testing
frontend_testing_enabled = False
browser = None
current_url = None

frontend_testing_enabled = False
browser = None
current_url = None

from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.common.exceptions import TimeoutException, WebDriverException

def check_url_accessibility(url, timeout=5):
    try:
        response = requests.get(url, timeout=timeout)
        return response.status_code == 200
    except requests.RequestException:
        return False

def diagnose_chrome_connection():
    try:
        response = requests.get("http://localhost:9222/json/version", timeout=5)
        if response.status_code == 200:
            print("Chrome DevTools is accessible.")
            return True
        else:
            print(f"Chrome DevTools returned unexpected status code: {response.status_code}")
    except requests.RequestException as e:
        print(f"Unable to connect to Chrome DevTools: {str(e)}")
    return False

def ensure_chrome_is_running():
    try:
        subprocess.check_output(["pgrep", "chrome"])
        print("Chrome is already running.")
    except subprocess.CalledProcessError:
        print("Chrome is not running. Starting Chrome with remote debugging...")
        
        display = os.environ.get('DISPLAY', ':0')
        os.environ['DISPLAY'] = display
        
        chrome_cmd = [
            "google-chrome",
            "--no-sandbox",
            "--remote-debugging-port=9222",
            "--user-data-dir=/tmp/chrome-testing"
        ]
        
        try:
            subprocess.Popen(chrome_cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            print(f"Started Chrome on display {display}")
            time.sleep(5)  # Wait for Chrome to start
        except Exception as e:
            print(f"Error starting Chrome: {str(e)}")
            print("Please ensure X11 forwarding is enabled in your SSH connection.")
            print("You may need to reconnect with: ssh -X user@host")
    
    if not diagnose_chrome_connection():
        print("Chrome DevTools is not accessible. Please check Chrome's status manually.")

from selenium.webdriver.chrome.options import Options

def setup_frontend_testing():
    global browser
    try:
        print("Setting up frontend testing...")

        # Set up Chrome options for connecting to the running instance
        chrome_options = Options()
        chrome_options.add_experimental_option("debuggerAddress", "127.0.0.1:9222")

        # Enable performance logging
        chrome_options.set_capability('goog:loggingPrefs', {'performance': 'ALL'})
        
        # Connect to the running Chrome instance
        service = Service()  # You might need to specify the path to chromedriver here
        browser = webdriver.Chrome(service=service, options=chrome_options)
        browser.execute_cdp_cmd('Network.enable', {})
        
    except Exception as e:
        print(f"Error setting up frontend testing: {str(e)}")
        print("\nTroubleshooting steps:")
        print("1. Ensure that Google Chrome is installed and running with remote debugging enabled.")
        print("2. Make sure no other Chrome instances are using the debugging port (9222).")
        print("3. If you're still having issues, try running the script with sudo:")
        print("   sudo python3 bootstrap.py --mode test --frontend")
        sys.exit(1)

def teardown_frontend_testing():
    global browser
    if browser:
        browser.quit()
        print("Chrome browser closed")

import random

def connect_to_chrome(max_retries=5, retry_delay=5):
    global browser
    for attempt in range(max_retries):
        try:
            chrome_options = Options()
            chrome_options.add_experimental_option("debuggerAddress", "127.0.0.1:9222")
            service = Service()
            browser = webdriver.Chrome(service=service, options=chrome_options)
            print("Connected to Chrome browser for UI testing")
            return True
        except WebDriverException as e:
            print(f"Attempt {attempt + 1} failed: {str(e)}")
            if attempt < max_retries - 1:
                print(f"Retrying in {retry_delay} seconds...")
                time.sleep(retry_delay)
            else:
                print("Max retries reached. Unable to connect to Chrome.")
                return False

def ui_open_url(url):
    global browser
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled.", False
    
    if not check_url_accessibility(url):
        return f"The URL {url} is not accessible. Please check if the server is running.", False
    
    try:
        if browser is None or not connect_to_chrome():
            return "Failed to connect to Chrome browser.", False
        
        browser.get(url)
        try:
            WebDriverWait(browser, 10).until(
                EC.presence_of_element_located((By.TAG_NAME, "body"))
            )
            return f"Opened URL: {url}", True
        except TimeoutException:
            return f"Timeout waiting for page to load: {url}", False
    except Exception as e:
        error_msg = f"Error opening URL: {str(e)}"
        print(error_msg)
        print("Attempting to reconnect to Chrome...")
        if connect_to_chrome():
            try:
                browser.get(url)
                WebDriverWait(browser, 10).until(
                    EC.presence_of_element_located((By.TAG_NAME, "body"))
                )
                return f"Reconnected and opened URL: {url}", True
            except Exception as e2:
                return f"{error_msg}\nReconnection attempt failed: {str(e2)}", False
        return error_msg, False

def ui_click_button(button_id):
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled.", False
    try:
        # Start capturing XHR requests if not already capturing
        capture_started = False
        if not xhr_capture_thread or not xhr_capture_thread.is_alive():
            ui_xhr_capture_start()
            capture_started = True

        # Clear console logs before clicking
        browser.get_log('browser')

        # Click the button
        button = WebDriverWait(browser, 10).until(
            EC.element_to_be_clickable((By.ID, button_id))
        )
        button.click()

        # Wait for 5 seconds to capture XHR requests and console logs
        time.sleep(5)

        # Capture console logs
        console_logs = browser.get_log('browser')

        # Stop capturing if we started it
        if capture_started:
            xhr_result, _ = ui_xhr_capture_stop()
        else:
            xhr_result = "XHR capture was already running."

        # Format console logs and limit to last 3000 characters
        formatted_logs = "\n".join([f"[{log['level']}] {log['message']}" for log in console_logs])
        formatted_logs = formatted_logs[-3000:]  # Limit to last 3000 characters

        return f"""Clicked button with ID: {button_id}

XHR Requests:
{xhr_result}

Console Logs (last 3000 characters):
{formatted_logs}""", True

    except Exception as e:
        ui_xhr_capture_stop()
        return f"Error clicking button: {str(e)}", False

def ui_check_element_text(element_id, expected_text):
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled."
    try:
        element = WebDriverWait(browser, 10).until(
            EC.presence_of_element_located((By.ID, element_id))
        )
        actual_text = element.text
        if actual_text == expected_text:
            return f"Element {element_id} has the expected text: {expected_text}"
        else:
            return f"Text mismatch for element {element_id}. Expected: {expected_text}, Actual: {actual_text}"
    except Exception as e:
        return f"Error checking element text: {str(e)}"

def ui_check_console_logs(expected_log):
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled.", False

    try:
        logs = browser.get_log('browser')
        full_log_text = "\n".join([log['message'] for log in logs])
        last_2000_chars = full_log_text[-2000:] if len(full_log_text) > 2000 else full_log_text
        
        expected_log_found = any(expected_log in log['message'] for log in logs)
        
        if expected_log_found:
            result = ""
            success = True
        else:
            result = ""
            success = False
        
        return f"{result}\n\nLast 2000 characters of logs:\n{last_2000_chars}", success
    except Exception as e:
        return f"Error checking console logs: {str(e)}\n\nUnable to retrieve logs.", False

import threading

# Global variables for XHR capture
xhr_capture_thread = None
xhr_capture_event = threading.Event()
captured_xhr_requests = []

def capture_xhr_requests():
    global captured_xhr_requests

    def request_will_be_sent(params):
        request = params['request']
        if request['method'] != 'OPTIONS':  # Filter out OPTIONS requests
            captured_xhr_requests.append({
                'url': request['url'],
                'method': request['method'],
                'timestamp': time.time()
            })

    def response_received(params):
        response = params['response']
        request = next((req for req in captured_xhr_requests if req['url'] == response['url']), None)
        if request:
            request['status'] = response['status']

    browser.add_cdp_event_listener('Network.requestWillBeSent', request_will_be_sent)
    browser.add_cdp_event_listener('Network.responseReceived', response_received)

    while not xhr_capture_event.is_set():
        time.sleep(0.1)

    browser.remove_cdp_listener('Network.requestWillBeSent', request_will_be_sent)
    browser.remove_cdp_listener('Network.responseReceived', response_received)

def ui_xhr_capture_start():
    global xhr_capture_thread, xhr_capture_event, captured_xhr_requests
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled.", False
    
    if xhr_capture_thread and xhr_capture_thread.is_alive():
        return "XHR capture is already running.", False
    
    xhr_capture_event.clear()
    captured_xhr_requests = []
    xhr_capture_thread = threading.Thread(target=capture_xhr_requests)
    xhr_capture_thread.start()
    return "XHR capture started.", True

def ui_xhr_capture_stop():
    global xhr_capture_thread, xhr_capture_event, captured_xhr_requests
    if not frontend_testing_enabled:
        return "Frontend testing is not enabled.", False
    
    if not xhr_capture_thread or not xhr_capture_thread.is_alive():
        return "No active XHR capture to stop.", False
    
    xhr_capture_event.set()
    xhr_capture_thread.join()
    xhr_capture_thread = None
    
    capture_result = json.dumps(captured_xhr_requests, indent=2)
    return f"XHR capture stopped. Captured requests:\n{capture_result}", True
    
def ensure_chrome_is_running():
    try:
        subprocess.check_output(["pgrep", "chrome"])
    except subprocess.CalledProcessError:
        print("Chrome is not running. Starting Chrome with remote debugging...")
        subprocess.Popen(["google-chrome", "--remote-debugging-port=9222", "--user-data-dir=/tmp/chrome-testing"])
        time.sleep(5)  # Wait for Chrome to start

def restart_chrome_if_needed():
    global browser
    try:
        # Try to execute a simple command
        browser.title
    except Exception:
        print("Chrome seems to have crashed. Restarting...")
        ensure_chrome_is_running()
        connect_to_chrome()
  
def handle_ui_action(command):
    global command_decisions, frontend_testing_enabled, current_url
    
    if frontend_testing_enabled:
        restart_chrome_if_needed()
        if command.upper().startswith("UI_OPEN:"):
            url = command.split(":", 1)[1].strip()
            return ui_open_url(url)
        elif command.upper().startswith("UI_CLICK:"):
            button_id = command.split(":", 1)[1].strip()
            return ui_click_button(button_id)
        elif command.upper().startswith("UI_CHECK_TEXT:"):
            parts = command.split(":", 2)
            element_id = parts[1].strip()
            expected_text = parts[2].strip()
            return ui_check_element_text(element_id, expected_text), True
        elif command.upper().startswith("UI_CHECK_LOG:"):
            expected_log = command.split(":", 1)[1].strip()
            return ui_check_console_logs(expected_log)
        # elif command.upper().startswith("UI_CHECK_XHR:"):
        #     parts = command.split(":", 3)
        #     expected_url = parts[1].strip() if len(parts) > 1 else None
        #     expected_method = parts[2].strip() if len(parts) > 2 else None
        #     return ui_check_xhr_requests(expected_url, expected_method)
        elif command.upper() == "UI_XHR_CAPTURE_START":
            return ui_xhr_capture_start()
        elif command.upper() == "UI_XHR_CAPTURE_STOP":
            return ui_xhr_capture_stop()
        else:
            return f"Unknown UI action: {command}", False
    else:
        return "Frontend testing is not enabled.", False
    
HasUserInterrupted = False
user_suggestion = ""

def generate_clean_tree(structure, indent=0):
    output = []
    
    # Handle files in current directory
    if "" in structure:
        for file in sorted(structure[""]):
            output.append("  " * indent + file)
    
    # Handle subdirectories
    for subdir in sorted(key for key in structure.keys() if key != ""):
        output.append("  " * indent + subdir + "/")
        output.extend(generate_clean_tree(structure[subdir], indent + 1))
    
    return output

def get_tree_structure():
    with open(PROJECT_STRUCTURE_FILE, 'r') as f:
        structure = json.load(f)
    
    output = ["<files>", "/"]
    output.extend(generate_clean_tree(structure, indent=1))
    output.append("</files>")
    
    return "\n".join(output)



    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
            
        if not commands:
            return file_content, "No valid modification commands found."
        
        # Apply the modifications
        modified_content, changes_summary = apply_modifications(file_content, commands)
        
        return modified_content, changes_summary
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"
    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
            
        if not commands:
            return file_content, "No valid modification commands found."
        
        # Apply the modifications
        modified_content, changes_summary = apply_modifications(file_content, commands)
        
        return modified_content, changes_summary
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"


    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
            
        if not commands:
            return file_content, "No valid commands found."
        
        # Apply the modifications
        modified_content, changes_summary = apply_modifications(file_content, commands)
        
        return modified_content, changes_summary
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"


    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
            
        if not commands:
            return file_content, "No valid modification commands found."
        
        # Apply the modifications
        modified_content, changes_summary = apply_modifications(file_content, commands)
        
        return modified_content, changes_summary
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"


    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
        
        # Apply the modifications
        return apply_modifications(file_content, commands)
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"


    """
    Process LLM's modification commands and apply them to the file content.
    Returns the modified content and a summary of changes.
    """
    try:
        # Parse the commands from the LLM response
        commands, error = parse_modification_commands(llm_response)
        
        if error:
            return file_content, error
            
        if not commands:
            return file_content, "No valid modification commands found."
        
        # Apply the modifications
        modified_content, changes_summary = apply_modifications(file_content, commands)
        
        return modified_content, changes_summary
        
    except Exception as e:
        return file_content, f"Error processing modifications: {str(e)}"


    """
    Process file modifications from LLM response.
    Returns the modified content and a summary of changes.
    """
    commands, error = parse_modification_commands(llm_response)
    if error:
        return file_content, error
    return apply_modifications(file_content, commands)


    """
    Process file modifications from LLM response.
    Returns the modified content and a summary of changes.
    """
    commands, error = parse_modification_commands(llm_response)
    if error:
        return file_content, error
    return apply_modifications(file_content, commands)

def parse_modification_commands(content):
    """
    Parse modification commands from LLM response.
    Enforces the rule that only one type of command can be used at a time.
    Returns a list of tuples: (command_type, start_line, end_line, new_content) or None if invalid
    """
    commands = []
    current_content = []
    current_command_type = None
    command_type = None
    line_range = None
    in_content_block = False
    
    # Empty content check
    if not content or not content.strip():
        return None, "Error: No valid modification commands found."

    #lines = content.split('\n')
    lines = content.splitlines(keepends=True)
    i = 0
    while i < len(lines):
        line = lines[i]
        command_line = line.strip()
        if not command_line and not in_content_block:
            i += 1
            continue
            
        # Check for new command
        if any(command_line.startswith(cmd) for cmd in ['ADD ', 'REMOVE ', 'MODIFY ']):
            parts = line.split(':', 1)
            command_parts = parts[0].strip().split()
            
            # Validate command format
            if len(command_parts) < 2:
                return None, "Error: No valid modification commands found."
                
            command_type = command_parts[0]
            if command_type not in ['ADD', 'REMOVE', 'MODIFY']:
                return None, "Error: No valid modification commands found."
            
            # Check if we're mixing command types
            if current_command_type and command_type != current_command_type:
                return None, f"Error: Cannot mix different command types. Found both {current_command_type} and {command_type}."
            
            current_command_type = command_type
            
            try:
                if command_type == 'ADD':
                    line_num = int(command_parts[1])
                    line_range = (line_num, line_num)
                    if len(parts) > 1:
                        if '<CONTENT_START>' not in parts[1]:
                            return None, "Error: No valid modification commands found."
                        content_part = parts[1].split('<CONTENT_START>', 1)[1]
                        if '<CONTENT_END>' in content_part:
                            content_part = content_part.split('<CONTENT_END>')[0]
                            commands.append((command_type, *line_range, content_part))
                            command_type = None
                            current_content = []
                            in_content_block = False
                        else:
                            current_content = [content_part]
                            in_content_block = True
                elif command_type == 'REMOVE':
                    if '-' in command_parts[1]:
                        line_range = tuple(map(int, command_parts[1].split('-')))
                    else:
                        line_num = int(command_parts[1])
                        line_range = (line_num, line_num)
                    commands.append(('REMOVE', line_range[0], line_range[1], ''))
                    command_type = None
                elif command_type == 'MODIFY':
                    if '-' in command_parts[1]:
                        line_range = tuple(map(int, command_parts[1].split('-')))
                    else:
                        line_num = int(command_parts[1])
                        line_range = (line_num, line_num)
                    if len(parts) > 1:
                        if '<CONTENT_START>' not in parts[1]:
                            return None, "Error: No valid modification commands found."
                        content_part = parts[1].split('<CONTENT_START>', 1)[1]
                        if '<CONTENT_END>' in content_part:
                            content_part = content_part.split('<CONTENT_END>')[0]
                            commands.append((command_type, *line_range, content_part))
                            command_type = None
                            current_content = []
                            in_content_block = False
                        else:
                            # Start with empty list since first content part might just have newline
                            current_content = []
                            if content_part:  # Only add if not empty
                                current_content.append(content_part)
                            in_content_block = True
            except (IndexError, ValueError):
                return None, "Error: No valid modification commands found."
            
        # Content collection for ADD and MODIFY
        elif current_content and in_content_block:
            if '<CONTENT_END>' in line:
                content_part = line.split('<CONTENT_END>')[0]
                current_content.append(content_part)
                content_text = ''.join(current_content)
                if content_text:
                    commands.append((command_type, *line_range, content_text))
                #current_content = []
                in_content_block = False
                command_type = None
            else:
                current_content.append(line)
        
        i += 1
    
    if not commands:
        return None, "Error: No valid modification commands found."
        
    return commands, None

def apply_modifications(file_content, commands):
    """
    Apply modification commands to the file content.
    Returns the modified content and a summary of changes.
    """
    if not commands:
        return file_content, "No valid modification commands found."
        
    lines = file_content.split('\n')
    changes_summary = []
    
    # All commands should be of the same type, so we can process them in order
    command_type = commands[0][0]
    line_adjustment = 0
    
    for cmd_type, start_line, end_line, content in commands:
        adjusted_start = start_line + line_adjustment
        adjusted_end = end_line + line_adjustment
        
        if cmd_type == 'REMOVE':
            # Ensure valid line numbers
            if 1 <= adjusted_start <= len(lines) and 1 <= adjusted_end <= len(lines):
                removed_lines = lines[adjusted_start-1:adjusted_end]
                del lines[adjusted_start-1:adjusted_end]
                line_adjustment -= (adjusted_end - adjusted_start + 1)
                changes_summary.append(f"Removed lines {start_line}-{end_line}:")
                changes_summary.extend(f"  - {line}" for line in removed_lines)
            else:
                changes_summary.append(f"Warning: Could not remove lines {start_line}-{end_line} (out of range)")
            
        elif cmd_type == 'MODIFY':
            # Ensure valid line numbers
            if 1 <= adjusted_start <= len(lines) and 1 <= adjusted_end <= len(lines):
                old_content = lines[adjusted_start-1:adjusted_end]
                new_lines = content.split('\n')
                lines[adjusted_start-1:adjusted_end] = new_lines
                line_adjustment += len(new_lines) - (adjusted_end - adjusted_start + 1)
                
                changes_summary.append(f"Modified lines {start_line}-{end_line}:")
                changes_summary.extend(f"  - Old: {line}" for line in old_content)
                changes_summary.extend(f"  + New: {line}" for line in new_lines)
            else:
                changes_summary.append(f"Warning: Could not modify lines {start_line}-{end_line} (out of range)")
            
        elif cmd_type == 'ADD':
            # Ensure valid line number
            if 0 <= adjusted_start <= len(lines):
                new_lines = content.split('\n')
                # For ADD commands, we need to insert after the specified line
                insert_pos = adjusted_start
                for new_line in new_lines:
                    lines.insert(insert_pos, new_line)
                    insert_pos += 1
                line_adjustment += len(new_lines)
                changes_summary.append(f"Added after line {start_line}:")
                changes_summary.extend(f"  + {line}" for line in new_lines)
            else:
                changes_summary.append(f"Warning: Could not add after line {start_line} (out of range)")
    
    return '\n'.join(lines), '\n'.join(changes_summary), False

def process_file_modifications(file_content, llm_response):
    """
    Process file modifications from LLM response.
    Returns the modified content and a summary of changes.
    """
    commands, error = parse_modification_commands(llm_response)
    #print(f"Commands:\n{commands}")
    if error:
        return file_content, error, error
    return apply_modifications(file_content, commands)

def test_and_debug_mode(llm_client):
    global unchanged_files, last_inspected_files, user_suggestion, WRITE_MODE, MAX_FILE_LENGTH

    JustStarted = True
    
    global COMMAND_HISTORY_FILE, TASK
    print(f"Created new command history file: {COMMAND_HISTORY_FILE}")

    # Add this function to handle unexpected terminations
    def handle_unexpected_termination(signum, frame):
        print("Unexpected termination detected. Saving current state...")
        save_command_history(command_history)
        sys.exit(1)

    # Register the signal handler
    signal.signal(signal.SIGTERM, handle_unexpected_termination)

    try:
        with open('project_summary.md', 'r') as f:
            project_summary = f.read()
    except FileNotFoundError:
        print("Warning: project_summary.md file not found. This file is important for providing context to the LLM.")
        print("For the current session, project summary to the LLM will be provided as 'No project summary found'")
        project_summary = "No project summary found"

    technical_brief = load_technical_brief()
    directory_summaries = technical_brief.get("directory_summaries", {})
    project_structure = read_project_structure()
    test_progress = load_test_progress()
    command_history = load_command_history()
    history_brief = load_history_brief()
    directory_tree_structure = get_tree_structure()

    
    print("Entering test and debug mode...")
    # print(f"Completed tests: {test_progress['completed_tests']}")
    # print(f"Current step: {test_progress['current_step']}")
    # print(f"Command history: {json.dumps(command_history, indent=2)}")
    print(f"History brief: {json.dumps(history_brief, indent=2)}")

    iteration = len(command_history) + 1
    start_iteration = iteration
    relative_iteration = 1
    last_unsuccessful_inspection_iteration = iteration

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
        global HasUserInterrupted, user_suggestion
        if HasUserInterrupted:
            print("\nSecond Ctrl+C received. Exiting the program.")
            kill_all_processes()
            sys.exit(0)
        HasUserInterrupted = True
        user_suggestion = handle_user_suggestion()

    # Set up the signal handler
    signal.signal(signal.SIGINT, handle_interrupt)

    retry_with_expert = False

    global unchanged_files, last_chat_content, chat_updated, chat_updated_iteration
    last_chat_content = read_chat_file()

    # Previous action analysis
    previous_action_analysis = None
    previous_file_diff = None
    ModifiedFile = False
    if user_suggestion != "":
        command_history.append({"user_message": user_suggestion})
        save_command_history(command_history)

    # Ask user for message on what user wants to do for this session
    if TASK:
        print(f"Task for this session: {TASK}")
        user_session_message = TASK
    else:
        # Ask user for task for this session since it's not specified in the command line.
        user_session_message = input("No task specified in the command line. What would you like to accomplish in this session? ")
    # Append this to the command history as the first message
    command_history.append({"user_message": user_session_message})
    save_command_history(command_history)
    
    while True:
        # Check for chat updates at the start of each iteration
        # check_all_processes()
        retry_with_expert = False
        relative_iteration = iteration - start_iteration + 1
        # if HasUserInterrupted:
        #     user_suggestion = handle_user_suggestion()

        # Update project structure after every iteration (need optimized)
        project_structure = generate_project_structure()
        save_project_structure(project_structure)
        directory_tree_structure = get_tree_structure()

        if check_chat_updates():
            chat_updated_iteration = iteration
            print("Chat file updated. Pausing...")
            wait_for_user_input()

        last_actions_context_count = 20
        last_n_iterations = get_last_n_iterations(command_history, last_actions_context_count)

        # Update history brief every 10 iterations
        if relative_iteration % UPDATE_INTERVAL == 9:
            history_brief = update_history_brief(command_history, history_brief, project_summary, last_chat_content, project_structure)
            save_history_brief(history_brief)
            print("Updated history brief.")

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
                process_outputs.append(f"Latest output from '{process_info['cmd']}':\n{output[-3000:]}")

        history_brief_prompt = get_history_brief_for_prompt(history_brief)

        prompt = f"""
You are in develop, test and debug mode for the project. You are a professional software architect, developer and tester. Adhere to the directives, best practices and provide accurate responses based on the project context. You can refer to the project summary, technical brief, and project structure for information.

<context>
<summary>
{project_summary}
</summary>

Project Structure (you're always in the root directory and cannot navigate to other directories, but can add cd <directory_path> to run commands that need to be run in a specific directory):
{directory_tree_structure}

<notes>
{last_chat_content}
</notes>

<history_brief>
{history_brief_prompt}
</history_brief>

<last_actions>
{json.dumps(last_n_iterations, indent=2)}
</last_actions>

<running_processes>
{f"Currently running processes (make sure the ones needed are running): {', '.join(process_status)}" if process_status else "No running processes."}
</running_processes>

<process_outputs>
Latest Process Outputs:
{', '.join(process_outputs) if process_outputs else "No new output from background processes."}
</process_outputs>

{"You modified a file in the previous iteration. If you are done with code changes and moving to testing, remember to start/restart the appropriate process using INDEF/RESTART " if ModifiedFile else ""}

{"This session just started, processes that were started in the previous session have been terminated." if JustStarted else ""}
</context>

<directives>
CRITICAL: Use the previous actions (especially the most recent action) and notes to learn from previous interactions and provide accurate responses. Avoid repeating the same actions. Additional importance to user suggestions.
0. Follow a continuous development, integration, and testing workflow. Do This includes writing code, testing, debugging, and fixing issues.
1. Put higher emphasis on the result/anlysis from the last iteration to make progress.
2. When doing development, consider reading multiple files to better integrate the current file with the rest of the project.
3. Never change code due to development environmental factors (ports, paths, etc.) unless explicitly mentioned in the prompt.
4. If there are environment related issue, use raw commands to fix them.
5. Use the files in the project structure to understand the context and provide accurate responses. Do not add new files.
6. Make sure that we're making progress with each step. If we go around in circles, assume that debug is wrong and start from the beginning.
7. Do not repeat the same action multiple times unless absolutely necessary.
8. RESTART a process after making changes to the code. This is crucial for the changes to take effect.
9. If something is not working, first assume that the process was not restarted after the code change or it has terminated unexpectedly. RESTART the process and check again.
</directives>

You can take the following actions:

1. Run a command/test from {', '.join(ALLOWED_COMMANDS)} or {', '.join(APPROVAL_REQUIRED_COMMANDS)} syncronously (blocking), use: "RUN: {', '.join(ALLOWED_COMMANDS)}". The script will wait for the command to finish and provide you with the output.
2. Run a command/test from {', '.join(ALLOWED_COMMANDS)} or {', '.join(APPROVAL_REQUIRED_COMMANDS)} asyncronously (non-blocking), use: "INDEF: <command>". This will run the command in the background and provide you with the initial output.
3. Run a raw command that requires approval, use: "RAW: <raw_command>". This will run the command in the shell and provide you with the output. You can use this for any command that is not in the allowed list.
4. Check the output of a running process using "CHECK: <command>"
5. Inspect up to four files in the project structure by replying with "INSPECT: <file_path>, <file_path>, ..." and get the analysis of the files based on the reason and goals.
6. Modify one file (should be one of the files being read) (maximum: 4) and read four files by replying with "READ: <file_path1>, <file_path2>, <file_path3>, <file_path4>; MODIFY: <file_path(1,2,3,4)>" 
7. Chat with the user for help or to give feedback by replying with "CHAT: <your question/feedback>". Do this when you see that no progress is being made.
8. Restart a running process with "RESTART: <command>"
9. Finish testing by replying with "DONE"
{f'''
10. UI Debugging and Testing Actions:
    - Open a URL: "UI_OPEN: <url>"
    - Check console logs (Used to debug and check if the page loaded correctly): "UI_CHECK_LOG: <expected_log_message>"
    - Click a button (with 5-second XHR capture): "UI_CLICK: <button_id>"
    - Start XHR network capture: "UI_XHR_CAPTURE_START"
    - Stop XHR network capture and get results: "UI_XHR_CAPTURE_STOP"

Current URL: {current_url if current_url else "No URL opened yet"}
''' if frontend_testing_enabled else ''}

{"{NEWLINE}Administrator suggestions for this action: " + user_suggestion + "{NEWLINE}" if HasUserInterrupted else ""}{"{NEWLINE}Previous action result/analysis/error: " + previous_action_analysis + "{NEWLINE}" if previous_action_analysis else ""}{"{NEWLINE}Previous file diff: " + previous_file_diff + "{NEWLINE}" if previous_file_diff else ""}{"{NEWLINE}" + Global_error + "{NEWLINE}" if Global_error else ""}

Provide your response in the following format:
ACTION: <your chosen action>
GOAL: <Provide this goal as context for when you're executing the actual command (max 80 words>
REASON: <Provide this as reason and context for when you're executing the actual command (max 80 words)>
<CoT>Your chain of thought for this action</CoT>
What would you like to do next to complete the user's task? Once the user task is accomplished, use "CHAT" to ask for feedback, if they say, there is nothing else to do, use "DONE". Use Chain of Thought (CoT) in project context, past actions/chat and directives to decide the next action. Think why the chosen action is the correct one, make surethat you've considered the directives and previous actions. Try to keep the response concise and to the point, helps save tokens.
        """
        # - Check element text (Use to debug contents of an element): "UI_CHECK_TEXT: <element_id>: <expected_text>"
        # if running_processes:
        #     final_prompt = prompt + prompt_prcoesses + prompt_extension
        # else:
        final_prompt = prompt # + prompt_extension

        HasUserInterrupted = False
        ModifiedFile = False
        previous_action_analysis = None
        previous_file_diff = None
        # For debug, print the process outputs been provided
        print(f"Running processes for debug: {process_status}")
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
        cot_match = re.search(r'<CoT>(.*?)</CoT>', response, re.DOTALL)
        command_entry = {"count": iteration}

        if action_match:
            action = action_match.group(1).strip()
            reason = reason_match.group(1).strip() if reason_match else "No reason provided"
            goals = goals_match.group(1).strip() if goals_match else "No goals provided"
            command_entry = {"count": iteration, "action": action, "reason": reason, "goal": goals}
            if user_suggestion != "":
                command_entry["user_message"] = user_suggestion
                user_suggestion = ""
                
            command_entry["process_outputs"] = process_outputs
            if JustStarted:
                command_entry["restart"] = "The session just started, processes that were started in the previous session have been terminated."


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

            elif action.upper().startswith("CHAT:"):
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
                    if set(inspect_files) == set(last_inspected_files) and (iteration - last_unsuccessful_inspection_iteration) == 1:
                        last_unsuccessful_inspection_iteration = iteration
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
<PREVIOUS_PROMPT_START>
{prompt}
<PREVIOUS_PROMPT_END>

This is the action executor system for your action selection as included before this text (only use the that as context and don't chose a action).

You chose to inspect the following files: {', '.join(inspect_files)}

Reason for this action: {reason}

Goals for this action: {goals}

Chain of Thought for this action: {cot_match}

Inspect for dependencies between the files. Check that variables, functions, parameters, and return values are used correctly and consistently across the files.

Inspected files:
                    """

                    for file_path, content in file_contents.items():
                        inspection_prompt += f"""
                        File: {file_path}
                        <FILE_CONTENT>
                        {content}
                        </FILE_CONTENT>
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

                Command history (last 10 commands) for better context: {json.dumps(last_n_iterations, indent=2)}

                Summarize the changes made to the file {file_path}. Compare the original content:
                {current_content}

                With the new content:
                {extracted_content}

                This is for the result section of this command. Provide a brief summary of the modifications in 50 words or less and if the goals were achieved.
                """
                changes_summary = llm_client.generate_response(changes_prompt, 1000)
                print(f"Changes summary:\n{changes_summary}")
                
                # technical_brief = update_technical_brief(file_path, extracted_content, iteration, mode="test", test_info=changes_summary)
                update_test_progress(current_step=f"Modified {file_path}")
                
                command_entry["result"] = {"changes_summary": changes_summary}

                # If change summary has "FILES ARE IDENTICAL", set retry_with_expert to True
                # if "FILES ARE IDENTICAL" in changes_summary:
                #     retry_with_expert = True
                #     # Change model to expert
                #     if isinstance(llm_client, VertexAILLM):
                #         llm_client.switch_model("claude-3-opus@20240229")
                #     continue

            elif action.upper().startswith("READ:"):
                parts = action.split(";")
                inspect_files = [f.strip() for f in parts[0].split(":")[1].split(",")]
                write_file = parts[1].split(":")[1].strip()

                # Check if the file is in the unchanged_files list and still under constraint
                if write_file in unchanged_files and unchanged_files[write_file] > 0:
                    error_msg = f"Error: The file {write_file} cannot be modified for {unchanged_files[write_file]} more iterations (this iteration won't count) due to no changes in the previous attempt. Use other actions such as INSPECT to increase the count."
                    previous_action_analysis = error_msg
                    command_entry["error"] = error_msg
                    print(error_msg)
                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue

                if write_file not in inspect_files:
                    error_msg = f"Error: The file to be written ({write_file}) must be one of the inspected files."
                    previous_action_analysis = error_msg
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
                        content = add_line_numbers(content)
                        content = truncate_content(content, MAX_FILE_LENGTH)
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
                        #update_project_structure(write_file)
                        print(f"Updated project structure with new file: {write_file}")
                    else:
                        error_msg = f"File did not exist. User denied creation of new file: {write_file}. You should work with existing files only."
                        previous_action_analysis = error_msg
                        command_entry["error"] = error_msg
                        print(error_msg)
                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue

                inspection_prompt = f"""
<PREVIOUS_PROMPT_START>
{prompt}
<PREVIOUS_PROMPT_END>

This is the action executor system for your action selection as appended before this text (only use the that as context and don't chose a action).

You chose to inspect multiple files and modify one of them.

Files to be thoroughly analysed and inspected to modify {write_file} file:
                """

                for file_path, content in file_contents.items():
                    inspection_prompt += f"""
<FILE_START({file_path})>
{content}
<FILE_END({file_path})>
"""
                print(f"WRITE_MODE: {WRITE_MODE}")
                if WRITE_MODE == "direct":
                    inspection_prompt += f"""
Use the contents of the provided files to modify the file {write_file}, consider the previous action, reason and goals for the modification. Use chain of thought to make the modifications.

{"Previous action result/analysis: " + previous_action_analysis if previous_action_analysis else ""}

Reason for this action: {reason}

Goals for this action: {goals}

Chain of Thought for this action: {cot_match}

Please provide the complete updated content for the file {write_file}, addressing any issues or improvements needed based on your inspection of all the files, while keeping code CONSISTENT across files, you must not make an unnecessary changes to the code. Never remove features unless specified. You must provide the full content since your output is directly written to the file without processing. Your output should be valid content for the file being written to. If you need to include any explanations, please do so as comments within the code. Remember, you're directly writing to the file!
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
                                    

                    previous_action_analysis = changes_summary
                    print(f"Changes summary:\n{changes_summary}")

                    changes_made, changes_made_diff = compare_and_write(write_file, extracted_content)
                    if not changes_made:
                        print("Warning: No actual changes were made in this iteration.")
                        command_entry["result"] = {"warning": "No actual changes were made in this iteration. Use INSPECT to check what changes are needed."}

                        # Add the file to unchanged_files with a counter of 2
                        unchanged_files[write_file] = 2

                        command_entry["error"] = f"The file {write_file} cannot be modified for the next 2 successful iterations due to no changes in this attempt. Use INSPECT to increase the count."

                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue
                    previous_file_diff = f"Changes for {write_file}:\n{changes_made_diff}"
                    changes_prompt = f"""
                    You are a professional software architect and developer.

                    You inspected multiple files and modified one of them. 

                    Reason given for this action: {reason}

                    Goals given for this action: {goals}

                    Chain of Thought for this action: {cot_match}

                    Command history (last 10 commands) for better context: {json.dumps(last_n_iterations, indent=2)}

                    Summarize the changes made to the file {write_file} for future notes to yourself. Compare the original content:
                    {read_file(write_file)}

                    With the new content:
                    {extracted_content}

                    This is for the result section of this command. Provide a brief summary of the modifications and if the goals were achieved in 100 words or less:
                    """
                    changes_summary = llm_client.generate_response(changes_prompt, 1000)
                    print(f"\nModified {write_file}")
                    ModifiedFile = True
                    
                    # technical_brief = update_technical_brief(write_file, new_content, iteration, mode="test", test_info=changes_summary)
                    update_test_progress(current_step=f"Inspected multiple files and modified {write_file}")
                    
                    command_entry["result"] = {"changes_summary": changes_summary}
                else:
                    inspection_prompt += f"""
Use the contents of the provided files to modify the file {write_file}, consider the previous action, reason and goals for the modification. Use chain of thought to make the modifications.
{"{NEWLINE}Previous action result/analysis: " + previous_action_analysis + "{NEWLINE}" if previous_action_analysis else ""}
Reason for this action: {reason}

Goals for this action: {goals}

Chain of Thought for this action: {cot_match}

You can modify the file by providing the changes using the following keywords. You can only use one distinct keyword at a time but okay use that distinct keyword multiple times. For example, you can use ADD keyword multiple times to add text multiple times but cannot use ADD and REMOVE or ADD and MODIFY or MODIFY and REMOVE in the same command:
To add any amount of text after a line number: ADD <line_number>:<CONTENT_START>new_content<CONTENT_END>
To remove lines, provide start and end line numbers, separated by a dash. If start and end line numbers are the same, a single line is removed: REMOVE <line_number_start>-<line_number_end>
To modify content, provide the line number range and the new content: MODIFY <line_number_start>-<line_number_end>:<CONTENT_START>new_content<CONTENT_END>

Remember to use ONLY ONE TYPE of keyword (only ADD(s) or only REMOVE(s) or only MODIFY(s)) and only provide the changes for the file to save tokens.
                    """
                    #print(f"Inspection prompt:\n{inspection_prompt}")
                    if retry_with_expert:
                        llm_response = llm_client.generate_response(inspection_prompt, 4096)
                    else:
                        llm_response = llm_client.generate_response(inspection_prompt, 8192)
                    #print(f"LLM response:\n{llm_response}")

                    # Process the modifications using existing functions
                    current_content = read_file(write_file)
                    modified_content, changes_summary, Error_in_modifications = process_file_modifications(current_content, llm_response)
                    if Error_in_modifications:
                        print(f"Error in modifications: {Error_in_modifications}")
                        command_entry["error"] = Error_in_modifications
                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue

                    changes_made, changes_made_diff = compare_and_write(write_file, modified_content)
                    
                    if not changes_made:
                        print("Warning: No actual changes were made in this iteration.")
                        command_entry["result"] = {"warning": "No actual changes were made in this iteration. Use INSPECT to check what changes are needed."}

                        # Add the file to unchanged_files with a counter of 2
                        unchanged_files[write_file] = 2

                        command_entry["error"] = f"The file {write_file} cannot be modified for the next 2 successful iterations due to no changes in this attempt. Use INSPECT to increase the count."

                        command_history.append(command_entry)
                        save_command_history(command_history)
                        iteration += 1
                        continue
                    previous_file_diff = f"Changes for {write_file}:\n{changes_made_diff}"
                    print(f"\nModified {write_file}")
                    ModifiedFile = True

                                                        
                    changes_prompt = f"""
You are a professional software architect and developer.

You inspected multiple files and modified one of them. 

Reason given for this action: {reason}

Goals given for this action: {goals}

Chain of Thought for this action: {cot_match}

Command history (last 10 commands) for better context: {json.dumps(last_n_iterations, indent=2)}

Summarize the changes made to the file {write_file} for future notes to yourself. Compare the original content:
{current_content}

With the new content:
{modified_content}

This is for the result section of this command. Provide a brief summary of the modifications and if the goals were achieved in 100 words or less:
                    """
                    changes_summary = llm_client.generate_response(changes_prompt, 1000)
                    previous_action_analysis = changes_summary
                    print(f"Changes summary:\n{changes_summary}")
                    
                    # technical_brief = update_technical_brief(write_file, new_content, iteration, mode="test", test_info=changes_summary)
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
                <PREVIOUS_PROMPT>
                {prompt}
                </PREVIOUS_PROMPT>

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
                # if the command is not in the ALLOWED_COMMANDS or APPROVAL_REQUIRED_COMMANDS, then it is not allowed to run
                if not any(action.startswith(cmd) for cmd in ALLOWED_COMMANDS + APPROVAL_REQUIRED_COMMANDS):
                    print(f"Command not allowed: {action}")
                    command_entry["error"] = f"Command not allowed: {action}. Please ask the user to add this command to the ALLOWED_COMMANDS list."
                    command_history.append(command_entry)
                    save_command_history(command_history)
                    iteration += 1
                    continue
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
                            <PREVIOUS_PROMPT>
                            {prompt}
                            </PREVIOUS_PROMPT>

                            You requested to run this command: {action}.

                            You gave this reason: {reason}

                            You set these goals: {goals}

                            Chain of Thought for this action: {cot_match}

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

            elif action.upper().startswith(("UI_OPEN:", "UI_CLICK:", "UI_CHECK_TEXT:", "UI_CHECK_LOG:")):
                print(f"\nExecuting UI action: {action}")
                output, success = handle_ui_action(action)
                print(f"Action output:\n{output}")
                update_test_progress(completed_test=action, current_step=f"Executed UI action: {action}")
                command_entry["output"] = output[:12000] if len(output) < 12000 else "Output truncated due to length"
                previous_action_analysis = output
                command_entry["success"] = success

                # Add UI-specific analysis
                ui_analysis_prompt = f"""
                <PREVIOUS_PROMPT>
                {prompt}
                </PREVIOUS_PROMPT>

                You executed a UI action: {action}

                You gave this reason: {reason}

                You set these goals: {goals}

                Chain of Thought for this action: {cot_match}
                
                The result was: {"successful" if success else "unsuccessful"}
                
                Output: {output}
                
                Based on this result, provide a brief analysis (max 100 words) of what happened and what should be done next in the UI testing process:
                """
                ui_analysis = llm_client.generate_response(ui_analysis_prompt, 1000)
                command_entry["ui_analysis"] = ui_analysis
                print(f"UI Action Analysis:\n{ui_analysis}")

            else:
                error_msg = f"Error: Invalid action: {action}"
                print(error_msg)
                command_entry["error"] = error_msg
            
            command_history.append(command_entry)
            save_command_history(command_history)
        else:
            print("Invalid response format. Please provide an action.")
            print("Raw response: ", response)
            command_entry["error"] = "Invalid response format. Please provide an action."
        
        JustStarted = False
        test_progress = load_test_progress()
        iteration += 1
        save_command_history(command_history)

        # Decrease the counter for unchanged files at the end of each iteration
        for file in list(unchanged_files.keys()):
            unchanged_files[file] -= 1
            if unchanged_files[file] == 0:
                del unchanged_files[file]

        #if retry_with_expert:
        #    # Switch back
        #    if isinstance(llm_client, VertexAILLM):
        #        llm_client.switch_model("claude-3-5-sonnet-20240620")
        #    retry_with_expert = False

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

    def handle_interrupt(signum, frame):
        print("\nCtrl+C received. Exiting the program.")
        sys.exit(0)

    # Set up the signal handler
    signal.signal(signal.SIGINT, handle_interrupt)

    try:
        with open('project_summary.md', 'r') as f:
            project_summary = f.read()
    except FileNotFoundError:
        print("Error: project_summary.md file not found. Please create this file as it is required to generate/update the project directory structure.")
        return

    current_structure = get_project_structure()

    user_input = input("\nDo you want to generate/update the project directory structure? (yes/no) [default: no]: ").lower()
    if user_input == 'yes':
        suggested_structure, explanation = review_project_structure(project_summary)

        if suggested_structure:
            print("Suggested project structure:")
            print(json.dumps(suggested_structure, indent=2))
            print("\nExplanation:")
            print(explanation)
            
            user_input = input("\nDo you want to apply the suggested structure? Warning: This will overwrite the current structure deleting all files and directories in the project directory (yes/no) [default: no]: ").lower()
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

    # Ask the user if they want to generate code/content for the files (default is no)
    user_input = input("\nDo you want to generate code/content for the files? [NOT RECOMMENDED as it is unreliable, use test mode instead to develop code/content for the project] (yes/no) [default: no]: ").lower()
    if user_input == '' or user_input == 'no':
        generate_code = False
    else:
        generate_code = True
    
    if generate_code:
        print("Generating code/content for the files...")
    else:
        print("Skipping code/content generation. Restart DevLM in test mode using --mode test.")
        exit()

    while not all_done and current_iteration < max_iterations:
        current_iteration += 1
        print(f"Starting iteration {current_iteration}")

        all_done = True
        # Generate project structure since files may have been deleted, added, or renamed
        project_structure = generate_project_structure()
        save_project_structure(project_structure)

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

def load_env_variables():
    env_file = 'devlm.env'
    if os.path.exists(env_file):
        with open(env_file, 'r') as f:
            for line in f:
                if line.strip() and not line.startswith('#'):
                    key, value = line.strip().split('=', 1)
                    print(f"Setting {key} to {value}")
                    os.environ[key] = value
    
    global MODEL, SOURCE, API_KEY, PROJECT_ID, REGION
    print(f"MODEL: {MODEL}, SOURCE: {SOURCE}, API_KEY: {API_KEY}, PROJECT_ID: {PROJECT_ID}, REGION: {REGION}")
    if MODEL.lower() == 'claude':
        if SOURCE.lower() == 'gcloud':
            if not PROJECT_ID or not REGION:
                PROJECT_ID = PROJECT_ID or os.environ.get('PROJECT_ID')
                REGION = REGION or os.environ.get('REGION')
                if not PROJECT_ID or not REGION:
                    print("Error: PROJECT_ID and REGION must be provided either as command-line arguments or set in the .env file when using Claude with Google Cloud.")
                    exit(1)
            print(f"Using Claude via Google Cloud. Project ID: {PROJECT_ID}, Region: {REGION}")
        elif SOURCE.lower() == 'anthropic':
            if not API_KEY:
                print("API_KEY is not set. Trying to set it from the .env file")
                API_KEY = API_KEY or os.environ.get('API_KEY')
                print(f"API_KEY: {API_KEY}")
            if not API_KEY:
                print("Error: API_KEY must be provided either as a command-line argument or set in the .env file when using Claude with Anthropic.")
                exit(1)
            print("Using Claude via Anthropic API.")
        else:
            print(f"Error: Invalid SOURCE '{SOURCE}' for MODEL 'claude'. Must be 'gcloud' or 'anthropic'.")
            exit(1)
    else:
        # load openai api key from .env file
        if SOURCE == 'openai':
            API_KEY = API_KEY or os.environ.get('API_KEY')
            if not API_KEY:
                print("Error: OPENAI_API_KEY must be provided either as a command-line argument or set in the .env file when using OpenAI.")
                exit(1)
        elif SOURCE.lower() == 'gcloud':
            if not PROJECT_ID or not REGION:
                PROJECT_ID = PROJECT_ID or os.environ.get('PROJECT_ID')
                REGION = REGION or os.environ.get('REGION')
                if not PROJECT_ID or not REGION:
                    print("Error: PROJECT_ID and REGION must be provided either as command-line arguments or set in the .env file when using Claude with Google Cloud.")
                    exit(1)
            print(f"Using Claude via Google Cloud. Project ID: {PROJECT_ID}, Region: {REGION}")
        elif SOURCE.lower() == 'anthropic':
            if not API_KEY:
                print("API_KEY is not set. Trying to set it from the .env file")
                API_KEY = API_KEY or os.environ.get('API_KEY')
                print(f"API_KEY: {API_KEY}")
            if not API_KEY:
                print("Error: API_KEY must be provided either as a command-line argument or set in the .env file when using Claude with Anthropic.")
                exit(1)
            print("Using Claude via Anthropic API.")

def main():
    global frontend_testing_enabled, browser, MODEL, SOURCE, API_KEY, PROJECT_ID, REGION, TASK, llm_client, WRITE_MODE, SERVER, DEBUG_PROMPT

    parser = argparse.ArgumentParser(description="DevLM Bootstrap script")
    parser.add_argument("--frontend", action="store_true", help="Enable frontend testing")
    parser.add_argument(
        "--mode", 
        choices=["test", "generate"],
        required=True,
        help="Specify the mode: 'test' or 'generate'."
    )
    parser.add_argument(
        "--model",
        default="claude",
        help="Specify the model to use (default: claude)"
    )
    parser.add_argument(
        "--source",
        choices=["gcloud", "anthropic", "openai"],
        default="anthropic",
        help="Specify the source for the model: 'gcloud' or 'anthropic' (default: anthropic)"
    )
    parser.add_argument(
        "--api-key",
        help="Specify the API key for Anthropic (only used if source is 'anthropic')"
    )
    parser.add_argument(
        "--project-id",
        help="Specify the Google Cloud project ID (only used if source is 'gcloud')"
    )
    parser.add_argument(
        "--region",
        help="Specify the Google Cloud region (only used if source is 'gcloud')"
    )
    parser.add_argument(
        "--server",
        default="https://api.openai.com/v1",
        help="Specify the server URL to use (only used if source is 'openai')"
    )
    parser.add_argument(
        "--project-path",
        default=".",
        help="Specify the path to the project directory (default: current directory)"
    )
    parser.add_argument(
        "--task",
        default=None,
        help="Specify the task to perform (default: None)"
    )
    parser.add_argument(
        "--write-mode",
        choices=["direct", "diff"],
        default="diff",
        help="Specify the write mode: 'direct' or 'diff' (default: diff)"
    )
    parser.add_argument(
        "--debug-prompt",
        action="store_true",
        help="Enable debug prompt mode"
    )
    args = parser.parse_args()

    MODEL = args.model
    SOURCE = args.source
    API_KEY = args.api_key
    PROJECT_ID = args.project_id
    REGION = args.region
    SERVER = args.server
    PROJECT_PATH = args.project_path
    TASK = args.task
    frontend_testing_enabled = args.frontend
    WRITE_MODE = args.write_mode
    DEBUG_PROMPT = args.debug_prompt
    # Load environment variables and validate settings
    load_env_variables()

    # Check all variables are set based on source and model
    if SOURCE == 'gcloud':
        if not PROJECT_ID or not REGION:
            print("Error: PROJECT_ID and REGION must be provided either as command-line arguments or set in the .env file when using Claude with Google Cloud.")
            exit(1)
    elif SOURCE == 'anthropic':
        if not API_KEY:
            print("Error: API_KEY must be provided either as a command-line argument or set in the .env file when using Claude with Anthropic.")
            exit(1)
    elif SOURCE == 'openai':
        if not API_KEY:
            print("Error: API_KEY must be provided either as a command-line argument or set in the .env file when using OpenAI.")
            exit(1)
        # if SERVER url is provided, check if it is valid, make sure it looks like a valid url
        if SERVER:
            if not SERVER.startswith("http://") and not SERVER.startswith("https://"):
                print(f"Error: Invalid SERVER URL '{SERVER}'. Please check the URL and try again.")
                exit(1)
    
    # Initialize the LLM client based on the source
    if SOURCE == 'gcloud':
        llm_client = get_llm_client("vertex_ai")  # or "anthropic" based on your preference
    elif SOURCE == 'anthropic':
        llm_client = get_llm_client("anthropic")
    elif SOURCE == 'openai':
        llm_client = get_llm_client("openai", model=MODEL)
    else:
        print(f"Error: Invalid SOURCE '{SOURCE}' for MODEL 'claude'. Must be 'gcloud' or 'anthropic'.")
        exit(1)

    # Change the working directory to the project path
    os.chdir(PROJECT_PATH)
    print(f"Working directory set to: {PROJECT_PATH}")

    # Ensure the devlm folder exists
    os.makedirs(DEVLM_FOLDER, exist_ok=True)
    if DEBUG_PROMPT:
        os.makedirs(os.path.join(DEVLM_FOLDER, "debug/prompts"), exist_ok=True)
    os.makedirs(os.path.join(DEVLM_FOLDER, "actions"), exist_ok=True)
    os.makedirs(os.path.join(DEVLM_FOLDER, "briefs"), exist_ok=True)

    if frontend_testing_enabled:
        ensure_chrome_is_running()
        if not connect_to_chrome():
            print("Failed to connect to Chrome. Please check Chrome's status manually and restart the script.")
            return
        setup_frontend_testing()
        atexit.register(teardown_frontend_testing)

    # Ensure chat file exists before entering any mode
    ensure_chat_file_exists()

    # Generate project structure since files may have been deleted, added, or renamed
    project_structure = generate_project_structure()
    save_project_structure(project_structure)
    print("Project directory structure has been generated.")

    if args.mode:
        mode = args.mode
    else:   
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
        # if frontend_testing_enabled:
        #     teardown_frontend_testing()
else:
    # For testing purposes
    __all__ = ['parse_changes', 'apply_changes']