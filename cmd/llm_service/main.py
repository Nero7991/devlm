import asyncio
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel, Field
from typing import List
import aiohttp
import os
import json
import logging
import subprocess
import tempfile
import requests
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
import redis.asyncio as redis

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

CLAUDE_API_KEY = os.getenv("CLAUDE_API_KEY", "YOUR_CLAUDE_API_KEY")
CLAUDE_API_URL = "https://api.anthropic.com/v1/messages"
SEARCH_API_KEY = os.getenv("SEARCH_API_KEY", "YOUR_SEARCH_API_KEY")
SEARCH_API_URL = "https://api.bing.microsoft.com/v7.0/search"
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379")

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

redis_client = redis.from_url(REDIS_URL)

class Message(BaseModel):
    role: str
    content: str

class Conversation(BaseModel):
    messages: List[Message]

class CodeExecutionRequest(BaseModel):
    code: str
    language: str = Field(default="python", regex="^(python|javascript|go)$")

async def call_claude(conversation: Conversation):
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": CLAUDE_API_KEY,
    }
    
    data = {
        "model": "claude-2",
        "messages": [{"role": msg.role, "content": msg.content} for msg in conversation.messages]
    }
    
    async with aiohttp.ClientSession() as session:
        try:
            async with session.post(CLAUDE_API_URL, headers=headers, json=data) as response:
                if response.status == 200:
                    return await response.json()
                elif response.status == 401:
                    raise HTTPException(status_code=401, detail="Unauthorized: Invalid API key")
                elif response.status == 429:
                    raise HTTPException(status_code=429, detail="Too many requests: Rate limit exceeded")
                else:
                    logger.error(f"Error calling Claude API: {response.status}")
                    raise HTTPException(status_code=response.status, detail=f"Error calling Claude API: {response.status}")
        except aiohttp.ClientError as e:
            logger.error(f"Network error calling Claude API: {str(e)}")
            raise HTTPException(status_code=503, detail="Service unavailable")

@app.post("/generate")
async def generate_code(conversation: Conversation):
    try:
        if not conversation.messages:
            raise HTTPException(status_code=400, detail="Conversation must contain at least one message")
        response = await call_claude(conversation)
        return JSONResponse(content={"generated_text": response['content'][0]['text']})
    except HTTPException as he:
        raise he
    except Exception as e:
        logger.error(f"Error in generate_code: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

@app.post("/analyze")
async def analyze_requirements(conversation: Conversation):
    try:
        if not conversation.messages:
            raise HTTPException(status_code=400, detail="Conversation must contain at least one message")
        response = await call_claude(conversation)
        return JSONResponse(content={"analysis": response['content'][0]['text']})
    except HTTPException as he:
        raise he
    except Exception as e:
        logger.error(f"Error in analyze_requirements: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

@app.post("/refine")
async def refine_code(conversation: Conversation):
    try:
        if not conversation.messages:
            raise HTTPException(status_code=400, detail="Conversation must contain at least one message")
        response = await call_claude(conversation)
        return JSONResponse(content={"refined_code": response['content'][0]['text']})
    except HTTPException as he:
        raise he
    except Exception as e:
        logger.error(f"Error in refine_code: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

@app.post("/execute")
async def execute_code(request: CodeExecutionRequest):
    try:
        if not request.code.strip():
            raise HTTPException(status_code=400, detail="Code cannot be empty")
        
        with tempfile.NamedTemporaryFile(mode='w', suffix=f'.{request.language}', delete=False) as temp_file:
            temp_file.write(request.code)
            temp_file_path = temp_file.name

        if request.language == "python":
            cmd = ['python', temp_file_path]
        elif request.language == "javascript":
            cmd = ['node', temp_file_path]
        elif request.language == "go":
            cmd = ['go', 'run', temp_file_path]
        else:
            raise HTTPException(status_code=400, detail="Unsupported language")

        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10, 
                                env={'PATH': '/usr/local/bin:/usr/bin:/bin'}, user='nobody')
        
        os.unlink(temp_file_path)

        return JSONResponse(content={
            "stdout": result.stdout,
            "stderr": result.stderr,
            "return_code": result.returncode
        })
    except subprocess.TimeoutExpired:
        return JSONResponse(content={"error": "Code execution timed out"}, status_code=408)
    except Exception as e:
        logger.error(f"Error in execute_code: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

@app.post("/search")
async def web_search(query: str, page: int = 1, results_per_page: int = 10):
    try:
        if not query.strip():
            raise HTTPException(status_code=400, detail="Search query cannot be empty")
        
        cache_key = f"search:{query}:{page}:{results_per_page}"
        cached_result = await redis_client.get(cache_key)
        if cached_result:
            return JSONResponse(content=json.loads(cached_result))

        headers = {"Ocp-Apim-Subscription-Key": SEARCH_API_KEY}
        params = {
            "q": query,
            "count": results_per_page,
            "offset": (page - 1) * results_per_page,
            "responseFilter": "Webpages"
        }
        response = requests.get(SEARCH_API_URL, headers=headers, params=params)
        response.raise_for_status()
        search_results = response.json()
        
        result = {
            "results": [
                {
                    "title": result["name"],
                    "url": result["url"],
                    "snippet": result["snippet"]
                }
                for result in search_results.get("webPages", {}).get("value", [])
            ],
            "total_results": search_results.get("webPages", {}).get("totalEstimatedMatches", 0),
            "current_page": page,
            "results_per_page": results_per_page
        }

        await redis_client.setex(cache_key, 3600, json.dumps(result))  # Cache for 1 hour
        return JSONResponse(content=result)
    except requests.exceptions.RequestException as e:
        logger.error(f"Error in web_search: {str(e)}")
        raise HTTPException(status_code=503, detail="Search service unavailable")
    except Exception as e:
        logger.error(f"Error in web_search: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Internal server error: {str(e)}")

@app.on_event("startup")
async def startup_event():
    logger.info("Starting LLM Service")
    await redis_client.ping()
    logger.info("Connected to Redis")

@app.on_event("shutdown")
async def shutdown_event():
    logger.info("Shutting down LLM Service")
    await redis_client.close()
    logger.info("Closed Redis connection")

@app.middleware("http")
async def log_requests(request: Request, call_next):
    request_id = request.headers.get("X-Request-ID", "N/A")
    logger.info(f"Incoming request: {request.method} {request.url} (Request ID: {request_id})")
    response = await call_next(request)
    logger.info(f"Response status: {response.status_code} (Request ID: {request_id})")
    return response

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)