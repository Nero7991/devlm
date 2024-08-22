# DevLM Architecture

## Overview

```go
// DevLM is an AI-powered software development assistant that uses Large Language Models (LLMs) to aid in code development.
// The system is designed to understand project requirements, generate code, run and test code, perform web searches when needed, and interact with the file system.
```

## Core Components

```go
// 1. API Gateway
// 2. Golang Backend Service
// 3. Python LLM Service
// 4. Action Executor
// 5. Code Execution Engine
// 6. Multiple Claude Sonnet Instances
```

## Supporting Services

```go
// 1. Redis Cache
// 2. PostgreSQL Database
```

## External Services

```go
// 1. Large Language Model API (Claude Sonnet)
// 2. Search Engine API
// 3. File System
```

## Component Interactions

### API Gateway

```go
import (
    "net/http"
    "github.com/dgrijalva/jwt-go"
    "golang.org/x/time/rate"
    "sync"
    "time"
)

var (
    limiter = rate.NewLimiter(rate.Limit(100), 10)
    ipLimiters = make(map[string]*rate.Limiter)
    ipLimitersMutex sync.Mutex
)

func getIPLimiter(ip string) *rate.Limiter {
    ipLimitersMutex.Lock()
    defer ipLimitersMutex.Unlock()

    limiter, exists := ipLimiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rate.Limit(10), 5)
        ipLimiters[ip] = limiter
    }

    return limiter
}

func API_Gateway(user_request *http.Request) (*http.Response, error) {
    token, err := validateJWTToken(user_request.Header.Get("Authorization"))
    if err != nil {
        return nil, err
    }

    ip := user_request.RemoteAddr
    ipLimiter := getIPLimiter(ip)
    if !ipLimiter.Allow() {
        return nil, errors.New("IP rate limit exceeded")
    }

    if !limiter.Allow() {
        return nil, errors.New("Global rate limit exceeded")
    }

    switch user_request.URL.Path {
    case "/backend":
        return routeToBackend(user_request)
    case "/llm":
        return routeToLLM(user_request)
    case "/execute":
        return routeToExecutionEngine(user_request)
    default:
        return nil, errors.New("invalid route")
    }
}

func validateJWTToken(tokenString string) (*jwt.Token, error) {
    return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte("your-secret-key"), nil
    })
}
```

### Golang Backend Service

```go
import (
    "log"
    "encoding/json"
    "runtime/debug"
)

func Golang_Backend_Service(request *http.Request) (*ProcessedRequest, error) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic occurred: %v\nStack trace: %s", r, debug.Stack())
        }
    }()

    var processedRequest ProcessedRequest
    err := json.NewDecoder(request.Body).Decode(&processedRequest)
    if err != nil {
        log.Printf("Error decoding request: %v", err)
        return nil, err
    }

    // Process the request
    err = processRequest(&processedRequest)
    if err != nil {
        log.Printf("Error processing request: %v", err)
        return nil, err
    }

    return &processedRequest, nil
}

func processRequest(req *ProcessedRequest) error {
    // Implement request processing logic
    return nil
}
```

### Python LLM Service

```python
import aiohttp
import asyncio
from functools import lru_cache
import hashlib
import json

async def Python_LLM_Service(task: str, context: dict) -> str:
    @lru_cache(maxsize=1000)
    async def cached_llm_request(task: str, context_hash: str) -> str:
        async with aiohttp.ClientSession() as session:
            async with session.post('https://api.anthropic.com/v1/complete', json={
                'prompt': f"{task}\nContext: {context}",
                'model': 'claude-v1',
                'max_tokens_to_sample': 1000
            }, headers={'Authorization': 'Bearer YOUR_API_KEY'}) as response:
                result = await response.json()
                return result['completion']

    context_hash = hashlib.md5(json.dumps(context, sort_keys=True).encode()).hexdigest()
    return await cached_llm_request(task, context_hash)
```

### Action Executor

```go
import (
    "os"
    "net/http"
    "io/ioutil"
)

func Action_Executor(action string, parameters map[string]interface{}) (map[string]interface{}, error) {
    switch action {
    case "file_read":
        return readFile(parameters["filename"].(string))
    case "file_write":
        return writeFile(parameters["filename"].(string), parameters["content"].(string))
    case "web_search":
        return webSearch(parameters["query"].(string))
    default:
        return nil, errors.New("unsupported action")
    }
}

func readFile(filename string) (map[string]interface{}, error) {
    content, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    return map[string]interface{}{"content": string(content)}, nil
}

func writeFile(filename string, content string) (map[string]interface{}, error) {
    err := ioutil.WriteFile(filename, []byte(content), 0644)
    if err != nil {
        return nil, err
    }
    return map[string]interface{}{"status": "success"}, nil
}

func webSearch(query string) (map[string]interface{}, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get("https://api.search.com?q=" + url.QueryEscape(query))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var result map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&result)
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

### Code Execution Engine

```go
import (
    "context"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    "io"
    "strings"
)

func Code_Execution_Engine(code string, language string) (map[string]interface{}, string, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, "", err
    }

    image, err := getDockerImage(language)
    if err != nil {
        return nil, "", err
    }

    resp, err := cli.ContainerCreate(ctx, &container.Config{
        Image: image,
        Cmd:   []string{language, "-c", code},
    }, &container.HostConfig{
        Resources: container.Resources{
            Memory:    50 * 1024 * 1024, // 50MB
            CPUPeriod: 100000,
            CPUQuota:  50000,
        },
    }, nil, nil, "")
    if err != nil {
        return nil, "", err
    }

    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        return nil, "", err
    }

    statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        return nil, "", err
    case <-statusCh:
    }

    out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
    if err != nil {
        return nil, "", err
    }

    var output strings.Builder
    io.Copy(&output, out)

    return map[string]interface{}{"status": "success"}, output.String(), nil
}

func getDockerImage(language string) (string, error) {
    switch language {
    case "python":
        return "python:3.9-slim", nil
    case "golang":
        return "golang:1.16", nil
    case "javascript":
        return "node:14-alpine", nil
    case "ruby":
        return "ruby:3.0-slim", nil
    default:
        return "", errors.New("unsupported language")
    }
}
```

### Redis Cache

```go
import (
    "github.com/go-redis/redis/v8"
    "time"
    "encoding/json"
)

var redisClient *redis.Client

func init() {
    redisClient = redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
}

func Redis_Cache(key string, value interface{}) (interface{}, error) {
    ctx := context.Background()

    cachedValue, err := redisClient.Get(ctx, key).Result()
    if err == nil {
        var result interface{}
        err = json.Unmarshal([]byte(cachedValue), &result)
        if err == nil {
            return result, nil
        }
    }

    jsonValue, err := json.Marshal(value)
    if err != nil {
        return nil, err
    }

    err = redisClient.Set(ctx, key, jsonValue, 30*time.Minute).Err()
    if err != nil {
        return nil, err
    }

    go evictOldEntries()

    return value, nil
}

func evictOldEntries() {
    ctx := context.Background()
    iter := redisClient.Scan(ctx, 0, "*", 0).Iterator()
    for iter.Next(ctx) {
        key := iter.Val()
        ttl, err := redisClient.TTL(ctx, key).Result()
        if err == nil && ttl < 5*time.Minute {
            redisClient.Del(ctx, key)
        }
    }
}
```

### PostgreSQL Database

```go
import (
    "github.com/jackc/pgx/v4/pgxpool"
    "context"
    "log"
)

var dbPool *pgxpool.Pool

func init() {
    var err error
    dbPool, err = pgxpool.Connect(context.Background(), "postgresql://user:password@localhost:5432/devlm")
    if err != nil {
        log.Fatalf("Unable to connect to database: %v", err)
    }
}

func PostgreSQL_Database(query string, parameters []interface{}) (pgx.Rows, error) {
    ctx := context.Background()
    rows, err := dbPool.Query(ctx, query, parameters...)
    if err != nil {
        return nil, err
    }
    return rows, nil
}

func CreateTables() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS projects (
            id SERIAL PRIMARY KEY,
            name TEXT NOT NULL,
            description TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS code_snippets (
            id SERIAL PRIMARY KEY,
            project_id INTEGER REFERENCES projects(id),
            language TEXT NOT NULL,
            content TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS execution_results (
            id SERIAL PRIMARY KEY,
            code_snippet_id INTEGER REFERENCES code_snippets(id),
            output TEXT,
            status TEXT,
            executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
    }

    ctx := context.Background()
    for _, query := range queries {
        _, err := dbPool.Exec(ctx, query)
        if err != nil {
            return err
        }
    }

    return nil
}
```