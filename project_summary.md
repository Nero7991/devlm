# LLM-based Software Developer (DevLM) Project Summary

## Project Overview
This project aims to create an AI-powered software development assistant that uses Large Language Models (LLMs) to aid in code development. The system will be able to understand project requirements, generate code, run and test code, perform web searches when needed, and interact with the file system.

## Key Features
1. Requirement analysis from a dev.txt file
2. Code generation and suggestion
3. Code execution and testing in a sandboxed environment
4. File system interactions (read/write operations)
5. Web search capabilities for additional information
6. Use of multiple Claude Sonnet instances for parallel task processing

## Architecture

### User Interface
- Simple text editor for writing project requirements in dev.txt
- No complex IDE features; focus on text-based interaction

### Core Components
1. API Gateway (Golang)
   - Handles incoming requests and routes them to appropriate services

2. Golang Backend Service
   - Manages core business logic and coordinates between components
   - Processes user requests and orchestrates tasks

3. Python LLM Service
   - Interfaces with Claude Sonnet instances
   - Handles LLM-related tasks (code generation, requirement analysis)

4. Action Executor (Golang)
   - Performs file operations and web searches
   - Executes system actions as needed

5. Code Execution Engine (Golang)
   - Runs and tests code in a secure, isolated environment
   - Provides feedback on code execution results

6. Multiple Claude Sonnet Instances
   - Used for parallel processing of subtasks
   - Coordinate through the Python LLM Service

### Supporting Services
1. Redis Cache
   - Stores frequently accessed data for quick retrieval

2. PostgreSQL Database
   - Persists project data, execution results, and system metadata

### External Services
1. Large Language Model API (Claude Sonnet)
2. Search Engine API
3. File System

## Workflow
1. User writes project requirements in dev.txt
2. System reads dev.txt and analyzes requirements
3. Tasks are distributed among multiple Claude Sonnet instances
4. Each instance generates code, suggests solutions, or provides analysis
5. Generated code is executed and tested in the sandbox environment
6. Results are aggregated and presented to the user
7. User can iteratively refine requirements and receive updated results

## Implementation Details
- Backend: Hybrid of Golang (for performance) and Python (for AI/ML tasks)
- Inter-service Communication: gRPC
- Code Execution: Isolated sandbox (e.g., Docker containers)
- Database: PostgreSQL with pgx (Golang) and asyncpg (Python)
- Caching: Redis with go-redis (Golang) and aioredis (Python)
- API Gateway: Custom implementation in Golang or using Traefik
- LLM Integration: Custom Python service using FastAPI

## Security Considerations
- Sandbox environment for code execution with resource limitations
- Input sanitization to prevent malicious code execution
- Restricted network access for executed code
- Secure API authentication and authorization

## Next Steps
1. Set up the basic project structure with Golang and Python services
2. Implement the file reading system for dev.txt
3. Create the LLM service to interface with Claude Sonnet instances
4. Develop the code execution engine with sandboxing
5. Implement the action executor for file and web operations
6. Set up the database and caching systems
7. Develop the API gateway and request routing
8. Integrate all components and implement the main workflow
9. Conduct thorough testing and security audits
10. Refine and optimize based on performance metrics

## Config

- Github: https://github.com/Nero7991/devlm