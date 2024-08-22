from fastapi import APIRouter, HTTPException, Depends, BackgroundTasks
from pydantic import BaseModel
from typing import List, Dict, Any
from llm_service.services.llm_manager import LLMManager
from llm_service.services.code_executor import CodeExecutor
from llm_service.services.file_manager import FileManager
from llm_service.services.task_scheduler import TaskScheduler
from llm_service.services.web_search import WebSearch
from llm_service.utils.cache import cache_result
from llm_service.utils.security import sanitize_input
from llm_service.utils.rate_limiter import rate_limit
from llm_service.utils.pagination import paginate_results
from llm_service.utils.error_handler import handle_exceptions
from llm_service.utils.logger import logger

router = APIRouter()

llm_manager = LLMManager()
code_executor = CodeExecutor()
file_manager = FileManager()
task_scheduler = TaskScheduler()
web_search = WebSearch()

class ProjectRequest(BaseModel):
    requirements: str

class CodeExecutionRequest(BaseModel):
    code: str

class WebSearchRequest(BaseModel):
    query: str

class FileOperationRequest(BaseModel):
    operation: str
    file_path: str
    content: str = None

class TaskScheduleRequest(BaseModel):
    tasks: List[Dict[str, Any]]

class TaskResultRequest(BaseModel):
    task_ids: List[str]
    page: int = 1
    page_size: int = 10

@router.post("/analyze_requirements")
@handle_exceptions
async def analyze_requirements(request: ProjectRequest, background_tasks: BackgroundTasks):
    try:
        sanitized_requirements = sanitize_input(request.requirements)
        analysis = await llm_manager.analyze_requirements(sanitized_requirements)
        scheduled_tasks = await task_scheduler.schedule_analysis_tasks(analysis)
        background_tasks.add_task(llm_manager.process_scheduled_tasks, scheduled_tasks)
        return {"analysis": analysis, "scheduled_tasks": scheduled_tasks}
    except (LLMManager.LLMError, TaskScheduler.SchedulingError) as e:
        logger.error(f"Error in analyze_requirements: {str(e)}")
        raise HTTPException(status_code=500, detail=str(e))
    except Exception as e:
        logger.critical(f"Unexpected error in analyze_requirements: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/generate_code")
@handle_exceptions
async def generate_code(request: ProjectRequest):
    try:
        sanitized_requirements = sanitize_input(request.requirements)
        code = await llm_manager.generate_code(sanitized_requirements)
        optimized_code = await llm_manager.optimize_code(code)
        security_check = await llm_manager.security_check(optimized_code)
        if security_check["passed"]:
            execution_result = await code_executor.execute(optimized_code)
            return {"code": optimized_code, "execution_result": execution_result}
        else:
            return {"code": optimized_code, "security_issues": security_check["issues"], "detailed_feedback": security_check["detailed_feedback"]}
    except LLMManager.LLMError as e:
        logger.error(f"LLM Error in generate_code: {str(e)}")
        raise HTTPException(status_code=500, detail=f"LLM Error: {str(e)}")
    except CodeExecutor.SecurityCheckError as e:
        logger.warning(f"Security Check Failed in generate_code: {str(e)}")
        raise HTTPException(status_code=400, detail=f"Security Check Failed: {str(e)}")
    except CodeExecutor.ExecutionError as e:
        logger.error(f"Execution Error in generate_code: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Execution Error: {str(e)}")
    except Exception as e:
        logger.critical(f"Unexpected error in generate_code: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/execute_code")
@handle_exceptions
async def execute_code(request: CodeExecutionRequest):
    try:
        sanitized_code = sanitize_input(request.code)
        result = await code_executor.execute_in_sandbox(sanitized_code, timeout=30, memory_limit=100_000_000)
        return {"result": result, "resource_usage": result.get("resource_usage", {})}
    except CodeExecutor.TimeoutError:
        logger.warning("Code execution timed out")
        raise HTTPException(status_code=408, detail="Code execution timed out")
    except CodeExecutor.SecurityViolationError as e:
        logger.warning(f"Security violation in execute_code: {str(e)}")
        raise HTTPException(status_code=400, detail=f"Security violation: {str(e)}")
    except CodeExecutor.ResourceLimitExceededError as e:
        logger.warning(f"Resource limit exceeded in execute_code: {str(e)}")
        raise HTTPException(status_code=413, detail=f"Resource limit exceeded: {str(e)}")
    except Exception as e:
        logger.critical(f"Unexpected error in execute_code: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/web_search")
@cache_result(expire=3600)
@rate_limit(max_calls=10, time_frame=60)
@handle_exceptions
async def perform_web_search(request: WebSearchRequest):
    try:
        sanitized_query = sanitize_input(request.query)
        results = await web_search.search(sanitized_query, depth=3)
        relevant_info = await llm_manager.analyze_search_results(results)
        return {"results": results, "relevant_info": relevant_info}
    except WebSearch.RateLimitError:
        logger.warning("Rate limit exceeded for web search")
        raise HTTPException(status_code=429, detail="Rate limit exceeded for web search")
    except WebSearch.SearchEngineError as e:
        logger.error(f"Search engine error in perform_web_search: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Search engine error: {str(e)}")
    except Exception as e:
        logger.critical(f"Unexpected error in perform_web_search: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/file_operation")
@handle_exceptions
async def file_operation(request: FileOperationRequest):
    try:
        sanitized_path = sanitize_input(request.file_path)
        sanitized_content = sanitize_input(request.content) if request.content else None
        if request.operation == "read":
            result = await file_manager.read_file(sanitized_path)
        elif request.operation == "write":
            result = await file_manager.write_file(sanitized_path, sanitized_content)
        elif request.operation == "delete":
            result = await file_manager.delete_file(sanitized_path)
        elif request.operation == "update":
            result = await file_manager.update_file(sanitized_path, sanitized_content)
        elif request.operation == "list":
            result = await file_manager.list_files(sanitized_path)
        else:
            raise ValueError("Invalid file operation")
        return {"result": result}
    except FileManager.FileNotFoundError:
        logger.warning(f"File not found: {sanitized_path}")
        raise HTTPException(status_code=404, detail="File not found")
    except FileManager.PermissionError:
        logger.warning(f"Permission denied for file operation: {sanitized_path}")
        raise HTTPException(status_code=403, detail="Permission denied for file operation")
    except FileManager.IOError as e:
        logger.error(f"IO Error in file_operation: {str(e)}")
        raise HTTPException(status_code=500, detail=f"IO Error: {str(e)}")
    except Exception as e:
        logger.critical(f"Unexpected error in file_operation: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/schedule_tasks")
@handle_exceptions
async def schedule_tasks(request: TaskScheduleRequest):
    try:
        sanitized_tasks = [sanitize_input(task) for task in request.tasks]
        scheduled_tasks = await task_scheduler.schedule_with_priority(sanitized_tasks)
        estimated_completion_times = await task_scheduler.estimate_completion_times(scheduled_tasks)
        return {"scheduled_tasks": scheduled_tasks, "estimated_completion_times": estimated_completion_times}
    except TaskScheduler.InvalidTaskError as e:
        logger.warning(f"Invalid task in schedule_tasks: {str(e)}")
        raise HTTPException(status_code=400, detail=f"Invalid task: {str(e)}")
    except TaskScheduler.SchedulerOverloadError:
        logger.warning("Task scheduler is overloaded")
        raise HTTPException(status_code=503, detail="Task scheduler is overloaded")
    except Exception as e:
        logger.critical(f"Unexpected error in schedule_tasks: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")

@router.post("/get_task_results")
@handle_exceptions
async def get_task_results(request: TaskResultRequest):
    try:
        sanitized_task_ids = [sanitize_input(task_id) for task_id in request.task_ids]
        results = await task_scheduler.get_results(sanitized_task_ids)
        analysis = await llm_manager.analyze_task_results(results)
        progress = await task_scheduler.get_progress(sanitized_task_ids)
        sorted_results = task_scheduler.sort_results(results, sort_by="completion_time", order="desc")
        filtered_results = task_scheduler.filter_results(sorted_results, status="completed")
        paginated_results = paginate_results(filtered_results, request.page, request.page_size)
        return {
            "results": paginated_results,
            "analysis": analysis,
            "progress": progress,
            "total_pages": len(filtered_results) // request.page_size + (1 if len(filtered_results) % request.page_size > 0 else 0),
            "current_page": request.page
        }
    except TaskScheduler.TaskNotFoundError as e:
        logger.warning(f"Task not found in get_task_results: {str(e)}")
        raise HTTPException(status_code=404, detail=f"Task not found: {str(e)}")
    except TaskScheduler.ResultsTooLargeError:
        logger.warning("Results too large to process in get_task_results")
        raise HTTPException(status_code=413, detail="Results too large to process")
    except Exception as e:
        logger.critical(f"Unexpected error in get_task_results: {str(e)}")
        raise HTTPException(status_code=500, detail="An unexpected error occurred")