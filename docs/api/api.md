# API Documentation

## Overview

This document provides an overview of the API endpoints for the DevLM project. The API is designed to facilitate communication between different components of the system, including the Golang backend service, Python LLM service, and the action executor.

## Base URL

All API requests should be prefixed with the following base URL:

```
https://api.devlm.example.com/v1
```

## Authentication

All API requests require authentication using a bearer token. Include the following header in your requests:

```
Authorization: Bearer <your_access_token>
```

## Endpoints

### Project Management

#### Create Project

```
POST /projects
```

Request body:
```json
{
  "name": "string",
  "description": "string",
  "language": "string",
  "framework": "string",
  "repository_url": "string",
  "team_size": "integer"
}
```

Response:
```json
{
  "project_id": "string",
  "name": "string",
  "description": "string",
  "language": "string",
  "framework": "string",
  "repository_url": "string",
  "team_size": "integer",
  "created_at": "string (ISO 8601 format)",
  "updated_at": "string (ISO 8601 format)"
}
```

#### Get Project

```
GET /projects/{project_id}
```

Response:
```json
{
  "project_id": "string",
  "name": "string",
  "description": "string",
  "language": "string",
  "framework": "string",
  "repository_url": "string",
  "team_size": "integer",
  "created_at": "string (ISO 8601 format)",
  "updated_at": "string (ISO 8601 format)"
}
```

#### Update Project

```
PUT /projects/{project_id}
```

Request body:
```json
{
  "name": "string",
  "description": "string",
  "language": "string",
  "framework": "string",
  "repository_url": "string",
  "team_size": "integer"
}
```

Response:
```json
{
  "project_id": "string",
  "name": "string",
  "description": "string",
  "language": "string",
  "framework": "string",
  "repository_url": "string",
  "team_size": "integer",
  "created_at": "string (ISO 8601 format)",
  "updated_at": "string (ISO 8601 format)"
}
```

#### Delete Project

```
DELETE /projects/{project_id}
```

Response:
```json
{
  "success": true,
  "message": "Project deleted successfully"
}
```

### Requirement Analysis

```
POST /projects/{project_id}/analyze-requirements
```

Request body:
```json
{
  "requirements": "string"
}
```

Response:
```json
{
  "analysis_results": {
    "tasks": [
      {
        "task_id": "string",
        "description": "string",
        "estimated_complexity": "integer",
        "estimated_time": "string (duration format)"
      }
    ],
    "dependencies": [
      {
        "name": "string",
        "version": "string"
      }
    ],
    "estimated_total_time": "string (duration format)"
  }
}
```

### Code Generation

```
POST /projects/{project_id}/generate-code
```

Request body:
```json
{
  "task_id": "string",
  "language": "string",
  "framework": "string",
  "code_style": "string"
}
```

Response:
```json
{
  "generated_code": "string",
  "explanation": "string",
  "suggestions": [
    {
      "description": "string",
      "code_snippet": "string"
    }
  ]
}
```

### Code Improvement

```
POST /projects/{project_id}/improve-code
```

Request body:
```json
{
  "code": "string",
  "language": "string",
  "improvement_focus": ["performance", "readability", "security"],
  "custom_rules": {
    "max_line_length": 100,
    "use_type_hints": true
  }
}
```

Response:
```json
{
  "improved_code": "string",
  "changes": [
    {
      "description": "string",
      "original": "string",
      "improved": "string"
    }
  ],
  "performance_impact": "string",
  "readability_score": "float"
}
```

### Code Execution

```
POST /projects/{project_id}/execute-code
```

Request body:
```json
{
  "code": "string",
  "language": "string",
  "input": "string",
  "timeout": 30,
  "memory_limit": 256
}
```

Response:
```json
{
  "output": "string",
  "execution_time": "float",
  "memory_usage": "float",
  "status": "success|error",
  "error_message": "string (if status is error)"
}
```

### Test Execution

```
POST /projects/{project_id}/run-tests
```

Request body:
```json
{
  "code": "string",
  "tests": [
    {
      "name": "string",
      "input": "string",
      "expected_output": "string"
    }
  ],
  "language": "string",
  "framework": "string"
}
```

Response:
```json
{
  "test_results": [
    {
      "name": "string",
      "status": "pass|fail",
      "actual_output": "string",
      "execution_time": "float"
    }
  ],
  "overall_status": "pass|fail",
  "total_tests": "integer",
  "passed_tests": "integer",
  "failed_tests": "integer"
}
```

### File Operations

#### Read File

```
GET /projects/{project_id}/files
```

Query parameters:
- `file_path`: string (required)
- `chunk_size`: integer (optional, default: 4096)

Response:
```json
{
  "file_content": "string",
  "encoding": "string",
  "size": "integer",
  "last_modified": "string (ISO 8601 format)"
}
```

#### Write File

```
POST /projects/{project_id}/files
```

Request body:
```json
{
  "file_path": "string",
  "content": "string",
  "encoding": "string",
  "overwrite": true
}
```

Response:
```json
{
  "success": true,
  "message": "File written successfully",
  "file_path": "string",
  "size": "integer"
}
```

### Web Search

```
GET /web-search
```

Query parameters:
- `query`: string (required)
- `num_results`: integer (optional, default: 10)
- `search_engine`: string (optional, default: "google")

Response:
```json
{
  "search_results": [
    {
      "title": "string",
      "url": "string",
      "snippet": "string"
    }
  ],
  "total_results": "integer",
  "search_time": "float"
}
```

## Error Handling

All endpoints return appropriate HTTP status codes. In case of an error, the response body will contain an error message:

```json
{
  "error": "string",
  "message": "string"
}
```

## Rate Limiting

API requests are rate-limited to 100 requests per minute per API key. The following headers are included in the response:

- `X-RateLimit-Limit`: The maximum number of requests you're permitted to make per minute.
- `X-RateLimit-Remaining`: The number of requests remaining in the current rate limit window.
- `X-RateLimit-Reset`: The time at which the current rate limit window resets in UTC epoch seconds.

## Versioning

The API uses semantic versioning. The current version is v1. When breaking changes are introduced, a new version will be released, and the old version will be supported for a grace period.

## Support

For API support, please contact api-support@devlm.example.com or visit our developer forum at https://developers.devlm.example.com.