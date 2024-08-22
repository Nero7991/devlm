# Coding Standards for DevLM Project

## Table of Contents
1. [General Guidelines](#general-guidelines)
2. [Golang Standards](#golang-standards)
3. [Python Standards](#python-standards)
4. [Documentation](#documentation)
5. [Testing](#testing)
6. [Version Control](#version-control)
7. [API Design](#api-design)
8. [Error Handling](#error-handling)

## General Guidelines

- Use consistent indentation (4 spaces for Python, tabs for Golang)
- Keep lines under 120 characters
- Use meaningful and descriptive names for variables, functions, and classes
- Follow the DRY (Don't Repeat Yourself) principle
- Write self-documenting code where possible
- Handle errors and exceptions appropriately

## Golang Standards

### Naming Conventions

- Use camelCase for variable and function names
- Use PascalCase for exported functions, types, and variables
- Use all-caps for constants

### Code Organization

- Group related declarations
- Order imports alphabetically, separating standard library and third-party imports

### Example

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	MaxRetries = 3
	BaseURL    = "https://api.example.com"
)

type User struct {
	ID   int
	Name string
}

func FormatName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

func ProcessUsers(users []User) (map[int]string, error) {
	if len(users) == 0 {
		return nil, errors.New("no users provided")
	}

	processedUsers := make(map[int]string)
	for _, user := range users {
		if user.Name == "" {
			return nil, errors.New("invalid user: empty name")
		}
		processedUsers[user.ID] = FormatName(user.Name)
	}
	return processedUsers, nil
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "Request timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, fmt.Sprintf("Error fetching user: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":%d,"name":"%s"}`, user.ID, user.Name)
}

func GetUserByID(ctx context.Context, id string) (*User, error) {
	var user *User
	var err error

	for i := 0; i < MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			user, err = fetchUserFromDatabase(id)
			if err == nil {
				return user, nil
			}
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return nil, errors.Wrap(err, "failed to fetch user after multiple attempts")
}

func fetchUserFromDatabase(id string) (*User, error) {
	// Simulating database fetch
	if id == "1" {
		return &User{ID: 1, Name: "John Doe"}, nil
	}
	return nil, errors.New("user not found")
}

func fetchData(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DisableCompression: false,
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and process the response body
	// ...

	return []byte("Sample data"), nil
}
```

## Python Standards

### Naming Conventions

- Use snake_case for variable and function names
- Use PascalCase for class names
- Use all-caps for constants

### Code Organization

- Use absolute imports
- Group imports in the order: standard library, third-party, local application
- Use type hints for function arguments and return values

### Example

```python
import logging
from typing import List, Dict, Optional, Any
from time import sleep
from functools import wraps

import requests
from pydantic import BaseModel, validator, Field

logger = logging.getLogger(__name__)

MAX_RETRIES = 3
BASE_URL = "https://api.example.com"

class User(BaseModel):
    id: int
    name: str = Field(..., min_length=1)
    email: str

    @validator('email')
    def validate_email(cls, v):
        if '@' not in v:
            raise ValueError('Invalid email format')
        return v

def format_name(name: str) -> str:
    return name.upper()

def process_users(users: List[User]) -> Dict[int, str]:
    if not users:
        raise ValueError("No users provided")
    
    try:
        return {user.id: format_name(user.name) for user in users}
    except ValueError as e:
        logger.error(f"Error processing users: {e}")
        raise

def retry_with_backoff(max_retries: int = MAX_RETRIES, base_delay: float = 1.0):
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except requests.RequestException as e:
                    if attempt == max_retries - 1:
                        logger.error(f"Error after {max_retries} attempts: {e}")
                        raise
                    delay = base_delay * (2 ** attempt)
                    logger.warning(f"Attempt {attempt + 1} failed, retrying in {delay:.2f} seconds")
                    sleep(delay)
        return wrapper
    return decorator

@retry_with_backoff()
def get_user(user_id: str) -> Optional[User]:
    response = requests.get(f"{BASE_URL}/users/{user_id}", timeout=5)
    response.raise_for_status()
    user_data = response.json()
    return User(**user_data)

def fetch_user_data(user_id: int) -> Dict[str, Any]:
    try:
        response = requests.get(f"{BASE_URL}/users/{user_id}", timeout=5)
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        logger.error(f"Failed to fetch user data: {e}")
        raise

def process_user(user_id: int) -> User:
    user_data = fetch_user_data(user_id)
    return User(**user_data)

# Example usage
if __name__ == "__main__":
    try:
        user = get_user("1")
        if user:
            print(f"User: {user}")
        else:
            print("User not found")
    except ValueError as e:
        print(f"Error: {e}")
    except requests.RequestException as e:
        print(f"Request error: {e}")
```

## Documentation

- Use clear and concise comments for complex logic
- Write docstrings for all public functions, classes, and modules
- Keep documentation up-to-date with code changes
- Use tools like Godoc for Golang and Sphinx for Python to generate documentation

## Testing

- Write unit tests for all functions and methods
- Aim for high test coverage (at least 80%)
- Use table-driven tests when appropriate
- Mock external dependencies in tests
- Run tests before committing code

## Version Control

- Use descriptive commit messages
- Create feature branches for new development
- Use pull requests for code reviews
- Keep commits small and focused
- Rebase feature branches before merging

## API Design

- Use RESTful principles for HTTP APIs
- Version your APIs
- Use consistent naming conventions for endpoints
- Provide clear documentation for all API endpoints
- Use appropriate HTTP status codes

## Error Handling

- Use custom error types when appropriate
- Log errors with context
- Return meaningful error messages to clients
- Avoid exposing sensitive information in error messages
- Use stack traces for debugging