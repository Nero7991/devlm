# DevLM: LLM-based Software Developer

## Project Overview

DevLM is an AI-powered software development assistant that uses Large Language Models (LLMs) to aid in code development. The system understands project requirements, generates code, runs and tests code, performs web searches when needed, and interacts with the file system.

## Key Features

- Requirement analysis from a dev.txt file
- Code generation and suggestion
- Code execution and testing in a sandboxed environment
- File system interactions (read/write operations)
- Web search capabilities for additional information
- Use of multiple Claude Sonnet instances for parallel task processing

## Architecture

### User Interface
- Simple text editor for writing project requirements in dev.txt
- Text-based interaction focus

### Core Components
1. API Gateway (Golang)
2. Golang Backend Service
3. Python LLM Service
4. Action Executor (Golang)
5. Code Execution Engine (Golang)
6. Multiple Claude Sonnet Instances

### Supporting Services
1. Redis Cache
2. PostgreSQL Database

### External Services
1. Large Language Model API (Claude Sonnet)
2. Search Engine API
3. File System

## Getting Started

### Prerequisites

- Go 1.21+
- Python 3.9+
- Docker and Docker Compose
- PostgreSQL
- Redis

### Installation

1. Clone the repository:
   ```
   git clone https://github.com/Nero7991/devlm.git
   cd devlm
   ```

2. Set up environment variables:
   ```
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. Build and run the services:
   ```
   make build
   make run
   ```

4. Access the API at `http://localhost:8080`

## Usage

1. Create a `dev.txt` file with your project requirements.
2. Use the API to analyze requirements, generate code, and execute tasks.
3. Interact with the system using the provided API endpoints.

### Example API Usage

```bash
# Analyze requirements
curl -X POST -H "Content-Type: application/json" -d '{"file": "dev.txt"}' http://localhost:8080/api/analyze

# Generate code
curl -X POST -H "Content-Type: application/json" -d '{"language": "python", "description": "Create a function to calculate fibonacci sequence"}' http://localhost:8080/api/generate

# Execute code
curl -X POST -H "Content-Type: application/json" -d '{"code": "print('Hello, DevLM!')"}' http://localhost:8080/api/execute
```

## Development

### Running Tests

```
make test
```

### Linting

```
make lint
```

### Formatting Code

```
make format
```

### Running Integration Tests

```
make integration-test
```

## Deployment

### Staging

```
make deploy-staging
```

### Production

```
make deploy-production
```

## Contributing

Please read [CONTRIBUTING.md](docs/development/contributing.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Claude Sonnet for powering the LLM capabilities
- The Go and Python communities for their excellent libraries and tools

## API Documentation

For detailed API documentation, please refer to our [API Documentation](docs/api/README.md) file.

## Core Functions

```python
import os
import time
import subprocess
from typing import Dict, Any
import docker
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

def analyze_requirements(file: str) -> Dict[str, Any]:
    if not os.path.exists(file):
        raise FileNotFoundError(f"The file {file} was not found.")

    with open(file, 'r') as f:
        content = f.read().strip()

    if not content:
        raise ValueError("The input file is empty.")

    # Implement actual requirement analysis logic using LLM
    llm_response = call_llm_api("analyze_requirements", content)
    
    analysis_result = {
        "requirements": llm_response["requirements"],
        "complexity": llm_response["complexity"],
        "technologies": llm_response["technologies"],
        "estimated_time": llm_response["estimated_time"],
        "potential_challenges": llm_response["potential_challenges"]
    }

    return analysis_result

def generate_code(language: str, description: str) -> str:
    supported_languages = ["python", "golang", "javascript", "java", "c++"]

    if language.lower() not in supported_languages:
        raise ValueError(f"Unsupported language: {language}. Supported languages are: {', '.join(supported_languages)}")

    if not description:
        raise ValueError("Description cannot be empty.")

    # Implement actual code generation logic using LLM
    llm_response = call_llm_api("generate_code", {"language": language, "description": description})
    
    return llm_response["generated_code"]

def execute_code(code: str) -> Dict[str, Any]:
    if not code.strip():
        raise ValueError("Code cannot be empty.")

    # Implement proper sandboxing using Docker
    client = docker.from_env()
    
    try:
        container = client.containers.run(
            "python:3.9-slim",
            ["python", "-c", code],
            detach=True,
            mem_limit="100m",
            network_mode="none",
            cpu_period=100000,
            cpu_quota=50000
        )
        
        start_time = time.time()
        container.wait(timeout=10)
        end_time = time.time()
        
        logs = container.logs().decode('utf-8')
        
        execution_result = {
            "output": logs,
            "errors": "",
            "execution_time": end_time - start_time,
            "exit_code": container.attrs['State']['ExitCode']
        }
        
        return execution_result
    except docker.errors.ContainerError as e:
        return {
            "output": "",
            "errors": str(e),
            "execution_time": 0,
            "exit_code": 1
        }
    finally:
        container.remove()

def deploy_staging() -> Dict[str, Any]:
    try:
        # Implement actual staging deployment logic
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = [
                executor.submit(prepare_staging_environment),
                executor.submit(build_project),
                executor.submit(run_tests),
                executor.submit(deploy_to_staging_servers),
                executor.submit(verify_deployment)
            ]
            
            for future in as_completed(futures):
                future.result()  # This will raise an exception if any step fails

        deployment_result = {
            "status": "success",
            "environment": "staging",
            "version": get_current_version(),
            "timestamp": time.time(),
            "details": "Deployment to staging completed successfully."
        }

        return deployment_result
    except Exception as e:
        raise RuntimeError(f"Staging deployment failed: {str(e)}")

def deploy_production() -> Dict[str, Any]:
    try:
        # Implement actual production deployment logic
        with ThreadPoolExecutor(max_workers=5) as executor:
            futures = [
                executor.submit(prepare_production_environment),
                executor.submit(build_project),
                executor.submit(run_comprehensive_tests),
                executor.submit(backup_production_data),
                executor.submit(deploy_to_production_servers),
                executor.submit(verify_deployment),
                executor.submit(run_smoke_tests)
            ]
            
            for future in as_completed(futures):
                future.result()  # This will raise an exception if any step fails

        deployment_result = {
            "status": "success",
            "environment": "production",
            "version": get_current_version(),
            "timestamp": time.time(),
            "details": "Deployment to production completed successfully."
        }

        return deployment_result
    except Exception as e:
        rollback_production()
        raise RuntimeError(f"Production deployment failed: {str(e)}")

def call_llm_api(endpoint: str, data: Any) -> Dict[str, Any]:
    # Implement the actual API call to the LLM service
    api_url = os.getenv("LLM_API_URL", "http://localhost:5000")
    response = requests.post(f"{api_url}/{endpoint}", json=data)
    response.raise_for_status()
    return response.json()

def get_current_version() -> str:
    # Implement version retrieval logic
    return "1.0.0"

def prepare_staging_environment():
    # Implement staging environment preparation
    pass

def build_project():
    # Implement project building logic
    pass

def run_tests():
    # Implement test running logic
    pass

def deploy_to_staging_servers():
    # Implement staging deployment logic
    pass

def verify_deployment():
    # Implement deployment verification logic
    pass

def prepare_production_environment():
    # Implement production environment preparation
    pass

def run_comprehensive_tests():
    # Implement comprehensive test suite
    pass

def backup_production_data():
    # Implement production data backup
    pass

def deploy_to_production_servers():
    # Implement production deployment logic
    pass

def run_smoke_tests():
    # Implement smoke tests
    pass

def rollback_production():
    # Implement production rollback logic
    pass

# Web-based user interface
# TODO: Implement a Flask-based web interface for easier interaction with DevLM

# Performance optimization
# TODO: Implement caching using Redis for frequently accessed data
# TODO: Implement parallel processing for large-scale projects

# Security enhancements
# TODO: Implement strict access controls for file system operations
# TODO: Enhance containerization security measures

# Advanced code generation
# TODO: Integrate with more advanced language models for code generation

# Language and framework support
# TODO: Add support for additional programming languages and frameworks

# Plugin system
# TODO: Implement a plugin architecture for extending DevLM functionality

# Version control and CI/CD integration
# TODO: Integrate with popular version control systems (e.g., Git)
# TODO: Implement CI/CD pipeline integration

# Test suite
# TODO: Develop a comprehensive test suite for all DevLM components

# User authentication and authorization
# TODO: Implement user authentication and role-based access control

# Documentation and tutorials
# TODO: Create detailed user and contributor documentation
# TODO: Develop interactive tutorials for DevLM usage
```

## Future Improvements

1. Implement a web-based user interface for easier interaction with the DevLM system.
2. Optimize performance for large-scale projects by implementing caching and parallel processing.
3. Enhance security measures for code execution and file system operations using containerization and strict access controls.
4. Implement more advanced code generation techniques using state-of-the-art language models.
5. Add support for more programming languages and frameworks.
6. Implement a plugin system for extending functionality.
7. Integrate with popular version control systems and CI/CD pipelines.
8. Develop a comprehensive test suite for all components of the system.
9. Implement user authentication and authorization for multi-user support.
10. Create detailed documentation and tutorials for users and contributors.