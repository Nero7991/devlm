from fastapi import FastAPI, HTTPException, BackgroundTasks, Depends
from pydantic import BaseModel
from typing import List, Dict, Any, Optional
import asyncio
import aiohttp
from llm_service.utils.llm_manager import LLMManager, LLMError
from llm_service.utils.task_scheduler import TaskScheduler, SchedulingError, InvalidTaskError, SchedulerOverloadError, TaskNotFoundError
from llm_service.utils.code_executor import CodeExecutor, ExecutionError, SecurityCheckError, TimeoutError, SecurityViolationError
from llm_service.utils.file_manager import FileManager, FileOperationError
from llm_service.utils.web_searcher import WebSearcher, RateLimitError, SearchEngineError
from llm_service.utils.result_analyzer import ResultAnalyzer, ResultsTooLargeError
from llm_service.utils.auth import get_current_user, User
import logging
from fastapi_limiter import FastAPILimiter
from fastapi_limiter.depends import RateLimiter
import redis.asyncio as redis

app = FastAPI()

# Initialize services
llm_manager = LLMManager()
task_scheduler = TaskScheduler()
code_executor = CodeExecutor()
file_manager = FileManager()
web_searcher = WebSearcher()
result_analyzer = ResultAnalyzer()

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Set up rate limiting
@app.on_event("startup")
async def startup():
    redis_instance = redis.from_url("redis://localhost", encoding="utf-8", decode_responses=True)
    await FastAPILimiter.init(redis_instance)

class ProjectRequest(BaseModel):
    project_id: str
    requirements: str

class CodeExecutionRequest(BaseModel):
    code: str

class WebSearchRequest(BaseModel):
    query: str

class FileOperationRequest(BaseModel):
    operation: str
    file_path: str
    content: Optional[str] = None

class TaskScheduleRequest(BaseModel):
    tasks: List[Dict[str, Any]]

class TaskResultRequest(BaseModel):
    task_ids: List[str]
    page: int = 1
    page_size: int = 10

@app.post("/analyze_requirements")
async def analyze_requirements(
    request: ProjectRequest,
    background_tasks: BackgroundTasks,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=5, seconds=60))
):
    try:
        analysis_result = await llm_manager.analyze_requirements(request.requirements)
        tasks = [
            {"type": "code_generation", "requirements": analysis_result},
            {"type": "test_generation", "requirements": analysis_result},
            {"type": "documentation", "requirements": analysis_result}
        ]
        scheduled_tasks = await task_scheduler.schedule(tasks)
        background_tasks.add_task(task_scheduler.process_tasks, scheduled_tasks)
        return {"analysis": analysis_result, "scheduled_tasks": scheduled_tasks}
    except LLMError as e:
        logger.error(f"LLM service error: {str(e)}", exc_info=True)
        raise HTTPException(status_code=503, detail="LLM service unavailable")
    except SchedulingError as e:
        logger.error(f"Task scheduling error: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail="Task scheduling failed")
    except Exception as e:
        logger.error(f"Unexpected error in analyze_requirements: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail="Internal server error")

@app.post("/generate_code")
async def generate_code(
    request: ProjectRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=3, seconds=60))
):
    try:
        generated_code = await llm_manager.generate_code(request.requirements)
        optimized_code = await llm_manager.optimize_code(generated_code)
        security_issues = await llm_manager.check_code_security(optimized_code)
        
        if security_issues:
            return {"code": optimized_code, "security_issues": security_issues}
        
        execution_result = await code_executor.execute(optimized_code)
        return {"code": optimized_code, "execution_result": execution_result}
    except LLMError as e:
        logger.error(f"LLM service error: {str(e)}", exc_info=True)
        raise HTTPException(status_code=503, detail="LLM service unavailable")
    except SecurityCheckError as e:
        logger.error(f"Code security check failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=400, detail=f"Code security check failed: {str(e)}")
    except ExecutionError as e:
        logger.error(f"Code execution failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Code execution failed: {str(e)}")
    except Exception as e:
        logger.error(f"Unexpected error in generate_code: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail="Internal server error")

@app.post("/execute_code")
async def execute_code(
    request: CodeExecutionRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=10, seconds=60))
):
    try:
        execution_result = await code_executor.execute_in_sandbox(request.code, timeout=30)
        return {"result": execution_result}
    except TimeoutError:
        logger.error("Code execution timed out", exc_info=True)
        raise HTTPException(status_code=408, detail="Code execution timed out")
    except SecurityViolationError as e:
        logger.error(f"Security violation in code execution: {str(e)}", exc_info=True)
        raise HTTPException(status_code=403, detail=f"Security violation in code execution: {str(e)}")
    except Exception as e:
        logger.error(f"Unexpected error in execute_code: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Code execution failed: {str(e)}")

@app.post("/perform_web_search")
async def perform_web_search(
    request: WebSearchRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=5, seconds=60))
):
    try:
        cached_results = await web_searcher.get_cached_results(request.query)
        if cached_results:
            return cached_results
        
        search_results = await web_searcher.search(request.query)
        relevant_info = await llm_manager.analyze_search_results(search_results)
        result = {"results": search_results, "relevant_info": relevant_info}
        await web_searcher.cache_results(request.query, result)
        return result
    except RateLimitError:
        logger.error("Rate limit exceeded for web search", exc_info=True)
        raise HTTPException(status_code=429, detail="Rate limit exceeded")
    except SearchEngineError:
        logger.error("Search engine unavailable", exc_info=True)
        raise HTTPException(status_code=503, detail="Search engine unavailable")
    except Exception as e:
        logger.error(f"Unexpected error in perform_web_search: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Web search failed: {str(e)}")

@app.post("/file_operation")
async def file_operation(
    request: FileOperationRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=20, seconds=60))
):
    try:
        if request.operation == "read":
            result = await file_manager.read_file(request.file_path)
        elif request.operation == "write":
            result = await file_manager.write_file(request.file_path, request.content)
        elif request.operation == "delete":
            result = await file_manager.delete_file(request.file_path)
        elif request.operation == "update":
            result = await file_manager.update_file(request.file_path, request.content)
        elif request.operation == "list":
            result = await file_manager.list_files(request.file_path)
        else:
            raise ValueError("Invalid file operation")
        return {"result": result}
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="File not found")
    except PermissionError:
        raise HTTPException(status_code=403, detail="Permission denied")
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except FileOperationError as e:
        logger.error(f"File operation error: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"File operation failed: {str(e)}")
    except Exception as e:
        logger.error(f"Unexpected error in file_operation: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail="Internal server error")

@app.post("/schedule_tasks")
async def schedule_tasks(
    request: TaskScheduleRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=10, seconds=60))
):
    try:
        scheduled_tasks = await task_scheduler.schedule_with_priority(request.tasks)
        return {"scheduled_tasks": scheduled_tasks}
    except InvalidTaskError as e:
        raise HTTPException(status_code=400, detail=f"Invalid task definition: {str(e)}")
    except SchedulerOverloadError:
        raise HTTPException(status_code=503, detail="Task scheduler overloaded")
    except Exception as e:
        logger.error(f"Unexpected error in schedule_tasks: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Task scheduling failed: {str(e)}")

@app.post("/get_task_results")
async def get_task_results(
    request: TaskResultRequest,
    current_user: User = Depends(get_current_user),
    rate_limiter: RateLimiter = Depends(RateLimiter(times=20, seconds=60))
):
    try:
        results, progress, total_pages = await task_scheduler.get_results_with_progress(
            request.task_ids, request.page, request.page_size
        )
        analyzed_results = await result_analyzer.analyze_task_results(results)
        return {
            "results": results,
            "analysis": analyzed_results,
            "progress": progress,
            "total_pages": total_pages,
            "current_page": request.page
        }
    except TaskNotFoundError as e:
        raise HTTPException(status_code=404, detail=f"Task not found: {str(e)}")
    except ResultsTooLargeError:
        raise HTTPException(status_code=413, detail="Results too large, use pagination")
    except Exception as e:
        logger.error(f"Unexpected error in get_task_results: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Failed to retrieve task results: {str(e)}")

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)