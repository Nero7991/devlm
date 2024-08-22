Here's the updated content for the file llm_service/utils/helpers.py, addressing todos and improving the code:

```python
import os
import json
import requests
import logging
import re
import time
from typing import Any, Dict, List, Optional
from bs4 import BeautifulSoup
from urllib.parse import urlparse
import hashlib
import redis
from functools import lru_cache

# Redis configuration
REDIS_HOST = 'localhost'
REDIS_PORT = 6379
REDIS_DB = 0
redis_client = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, db=REDIS_DB)

def read_dev_file(file_path: str, encoding: str = 'utf-8') -> str:
    try:
        with open(file_path, 'r', encoding=encoding) as file:
            return file.read()
    except FileNotFoundError:
        raise FileNotFoundError(f"Dev file not found at {file_path}")
    except IOError as e:
        raise IOError(f"Error reading file {file_path}: {str(e)}")
    except Exception as e:
        raise Exception(f"Unexpected error reading file {file_path}: {str(e)}")

def parse_requirements(content: str) -> List[str]:
    requirements = []
    for line in content.split('\n'):
        line = line.strip()
        if line and not line.startswith('#'):
            parts = re.split(r'([=<>~!]=?|\s)', line, 1)
            requirement = parts[0].strip()
            if len(parts) > 1:
                requirement += f' {parts[1].strip()}{parts[2].strip() if len(parts) > 2 else ""}'
            requirements.append(requirement)
    return requirements

def execute_web_search(query: str) -> Dict[str, Any]:
    config = load_project_config()
    api_key = config.get('google_api_key', 'YOUR_GOOGLE_API_KEY')
    search_engine_id = config.get('search_engine_id', 'YOUR_SEARCH_ENGINE_ID')
    url = f"https://www.googleapis.com/customsearch/v1?key={api_key}&cx={search_engine_id}&q={query}"
    
    cache_key = hashlib.md5(query.encode()).hexdigest()
    cached_result = get_cached_result(cache_key)
    if cached_result:
        return cached_result

    try:
        response = requests.get(url, timeout=config.get('timeout', 30))
        response.raise_for_status()
        result = response.json()
        cache_result(cache_key, result)
        return result
    except requests.RequestException as e:
        log_error(f"Error executing web search: {str(e)}")
        return {}

def save_generated_code(code: str, file_path: str, mode: str = 'w') -> None:
    try:
        os.makedirs(os.path.dirname(file_path), exist_ok=True)
        with open(file_path, mode) as file:
            file.write(code)
        
        if mode == 'w':
            if file_path.endswith('.py'):
                try:
                    import autopep8
                    formatted_code = autopep8.fix_code(code)
                    with open(file_path, 'w') as file:
                        file.write(formatted_code)
                except ImportError:
                    log_error("autopep8 not installed. Skipping Python code formatting.")
            elif file_path.endswith('.js'):
                try:
                    import jsbeautifier
                    formatted_code = jsbeautifier.beautify(code)
                    with open(file_path, 'w') as file:
                        file.write(formatted_code)
                except ImportError:
                    log_error("jsbeautifier not installed. Skipping JavaScript code formatting.")
    except IOError as e:
        raise IOError(f"Error writing to file {file_path}: {str(e)}")

@lru_cache(maxsize=1)
def load_project_config() -> Dict[str, Any]:
    config_path = os.path.join(os.path.dirname(__file__), '..', 'config', 'project_config.json')
    try:
        with open(config_path, 'r') as config_file:
            config = json.load(config_file)
        validate_config(config)
        return config
    except FileNotFoundError:
        raise FileNotFoundError(f"Config file not found at {config_path}")
    except json.JSONDecodeError as e:
        raise json.JSONDecodeError(f"Invalid JSON in config file: {str(e)}", e.doc, e.pos)

def validate_config(config: Dict[str, Any]) -> None:
    required_keys = ['google_api_key', 'search_engine_id', 'log_level']
    for key in required_keys:
        if key not in config:
            raise ValueError(f"Missing required configuration key: {key}")
    
    default_values = {
        'max_retries': 3,
        'timeout': 30,
        'cache_expiration': 3600,
        'config_version': '1.0'
    }
    for key, value in default_values.items():
        if key not in config:
            config[key] = value
    
    if not isinstance(config.get('max_retries'), int) or config['max_retries'] < 0:
        raise ValueError("max_retries must be a non-negative integer")
    if not isinstance(config.get('timeout'), (int, float)) or config['timeout'] <= 0:
        raise ValueError("timeout must be a positive number")
    if not isinstance(config.get('cache_expiration'), int) or config['cache_expiration'] < 0:
        raise ValueError("cache_expiration must be a non-negative integer")
    if not isinstance(config.get('config_version'), str):
        raise ValueError("config_version must be a string")

def sanitize_input(input_str: str) -> str:
    sanitized = re.sub(r'[;\'\"\\]', '', input_str)
    sanitized = re.sub(r'\b(SELECT|INSERT|UPDATE|DELETE|DROP|UNION|FROM|WHERE|EXEC|EXECUTE|TRUNCATE|ALTER|CREATE|TABLE|DATABASE)\b', '', sanitized, flags=re.IGNORECASE)
    sanitized = re.sub(r'(\%27)|(\-\-)|(\%23)|(#)', '', sanitized)
    return sanitized

def log_error(error_message: str, level: str = 'ERROR', log_file: Optional[str] = None) -> None:
    config = load_project_config()
    log_level = getattr(logging, config.get('log_level', 'ERROR').upper())
    log_format = '%(asctime)s - %(levelname)s - %(message)s'
    
    if log_file:
        logging.basicConfig(level=log_level, format=log_format, filename=log_file, filemode='a')
    else:
        logging.basicConfig(level=log_level, format=log_format)
    
    getattr(logging, level.lower())(error_message)

def create_directory_if_not_exists(directory_path: str, mode: int = 0o755, recursive: bool = False) -> None:
    if not os.path.exists(directory_path):
        try:
            if recursive:
                os.makedirs(directory_path, mode=mode, exist_ok=True)
            else:
                os.mkdir(directory_path, mode=mode)
        except OSError as e:
            if e.errno != errno.EEXIST:
                raise OSError(f"Error creating directory {directory_path}: {str(e)}")

def get_file_content(file_path: str, encoding: str = 'utf-8') -> Optional[str]:
    max_retries = load_project_config().get('max_retries', 3)
    for attempt in range(max_retries):
        try:
            with open(file_path, 'r', encoding=encoding) as file:
                return file.read()
        except FileNotFoundError:
            return None
        except UnicodeDecodeError:
            raise UnicodeDecodeError(f"Error decoding file {file_path} with encoding {encoding}")
        except IOError as e:
            if attempt == max_retries - 1:
                raise IOError(f"Error reading file {file_path} after {max_retries} attempts: {str(e)}")
            else:
                time.sleep(0.5 * (attempt + 1))  # Exponential backoff

def make_api_request(url: str, method: str = 'GET', data: Optional[Dict[str, Any]] = None, headers: Optional[Dict[str, str]] = None, auth: Optional[tuple] = None) -> Dict[str, Any]:
    config = load_project_config()
    timeout = config.get('timeout', 30)
    max_retries = config.get('max_retries', 3)
    
    for attempt in range(max_retries):
        try:
            response = requests.request(method, url, json=data, headers=headers, auth=auth, timeout=timeout)
            response.raise_for_status()
            return response.json()
        except requests.RequestException as e:
            if attempt == max_retries - 1:
                raise requests.RequestException(f"Error making API request after {max_retries} attempts: {str(e)}")
            else:
                time.sleep(0.5 * (attempt + 1))  # Exponential backoff

def extract_text_from_html(html_content: str, preserve_elements: Optional[List[str]] = None) -> str:
    soup = BeautifulSoup(html_content, 'html.parser')
    if preserve_elements:
        for element in preserve_elements:
            for tag in soup.find_all(element):
                tag.replace_with(f' {tag.get_text()} ')
    
    for tag in soup.find_all(True):
        if tag.name not in (preserve_elements or []):
            tag.replace_with(tag.get_text())
    
    return soup.get_text(separator=' ', strip=True)

def validate_file_extension(file_path: str, allowed_extensions: List[str], case_sensitive: bool = False) -> bool:
    _, extension = os.path.splitext(file_path)
    if not case_sensitive:
        extension = extension.lower()
        allowed_extensions = [ext.lower() for ext in allowed_extensions]
    return extension in [ext if ext.startswith('.') else f'.{ext}' for ext in allowed_extensions]

def get_file_size(file_path: str, unit: str = 'bytes') -> float:
    try:
        size_bytes = os.path.getsize(file_path)
        units = {
            'bytes': 1,
            'KB': 1024,
            'MB': 1024 * 1024,
            'GB': 1024 * 1024 * 1024,
            'TB': 1024 * 1024 * 1024 * 1024
        }
        if unit not in units:
            raise ValueError("Invalid unit. Use 'bytes', 'KB', 'MB', 'GB', or 'TB'.")
        return size_bytes / units[unit]
    except OSError:
        return -1

def is_valid_url(url: str) -> bool:
    try:
        result = urlparse(url)
        return all([result.scheme, result.netloc]) and result.scheme in ['http', 'https', 'ftp', 'sftp'] and '.' in result.netloc
    except ValueError:
        return False

def get_cached_result(key: str) -> Optional[Dict[str, Any]]:
    cached_data = redis_client.get(key)
    if cached_data:
        return json.loads(cached_data)
    return None

def cache_result(key: str, result: Dict[str, Any]) -> None:
    config = load_project_config()
    expiration = config.get('cache_expiration', 3600)
    redis_client.setex(key, expiration, json.dumps(result))
```