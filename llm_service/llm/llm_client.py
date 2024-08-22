Here's the updated content for the file llm_service/llm/llm_client.py:

```python
import aiohttp
import asyncio
import os
from typing import Dict, Any, List, Type, TracebackType
import logging
import time
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type
import json
from prometheus_client import Counter, Histogram
from cachetools import TTLCache
from prompt_generator import PromptGenerator

class LLMClient:
    def __init__(self, api_key: str, base_url: str):
        if not api_key or not base_url:
            raise ValueError("API key and base URL must be provided")
        self.api_key = api_key
        self.base_url = base_url
        self.session = None
        self.logger = logging.getLogger(__name__)
        self.cache = TTLCache(maxsize=1000, ttl=3600)  # Cache with 1-hour TTL
        self.request_counter = Counter('llm_api_requests_total', 'Total number of LLM API requests', ['endpoint'])
        self.request_latency = Histogram('llm_api_request_latency_seconds', 'LLM API request latency in seconds', ['endpoint'])
        self.prompt_generator = PromptGenerator("You are an AI assistant specialized in software development tasks.")

    async def __aenter__(self):
        try:
            self.session = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=30))
        except Exception as e:
            self.logger.error(f"Failed to create aiohttp ClientSession: {e}")
            raise
        return self

    async def __aexit__(
        self,
        exc_type: Type[BaseException] | None,
        exc_val: BaseException | None,
        exc_tb: TracebackType | None,
    ):
        if self.session:
            try:
                await self.session.close()
            except Exception as e:
                self.logger.error(f"Error closing aiohttp ClientSession: {e}")

    @retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=4, max=10),
           retry=retry_if_exception_type((aiohttp.ClientError, asyncio.TimeoutError)))
    async def generate_code(self, requirements: str, language: str = "python", code_style: str = "pep8") -> str:
        prompt = self.prompt_generator.generate_code_prompt(requirements, language, code_style)
        endpoint = f"{self.base_url}/generate"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        data = {"prompt": prompt}

        self.request_counter.labels(endpoint='generate').inc()
        with self.request_latency.labels(endpoint='generate').time():
            try:
                async with self.session.post(endpoint, headers=headers, json=data) as response:
                    response.raise_for_status()
                    result = await response.json()
                    generated_code = self.prompt_generator.extract_code_from_response(result.get("generated_code", ""))
                    return generated_code.get(language, "")
            except (aiohttp.ClientError, asyncio.TimeoutError) as e:
                self.logger.error(f"API request failed: {e}")
                raise

    @retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=4, max=10),
           retry=retry_if_exception_type((aiohttp.ClientError, asyncio.TimeoutError)))
    async def analyze_requirements(self, requirements: str, categories: List[str] = None) -> Dict[str, Any]:
        prompt = self.prompt_generator.generate_analysis_prompt(requirements, categories)
        endpoint = f"{self.base_url}/analyze"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        data = {"prompt": prompt}

        self.request_counter.labels(endpoint='analyze').inc()
        with self.request_latency.labels(endpoint='analyze').time():
            try:
                async with self.session.post(endpoint, headers=headers, json=data) as response:
                    response.raise_for_status()
                    result = await response.json()
                    analysis = self.prompt_generator.parse_analysis_response(result.get("analysis", ""))
                    self._validate_analysis_response(analysis)
                    return analysis
            except (aiohttp.ClientError, asyncio.TimeoutError) as e:
                self.logger.error(f"API request failed: {e}")
                raise
            except ValueError as e:
                self.logger.error(f"Invalid API response: {e}")
                raise

    def _validate_analysis_response(self, response: Dict[str, Any]):
        required_keys = ["main_features", "challenges", "architecture", "timeline"]
        for key in required_keys:
            if key not in response:
                raise ValueError(f"Missing required key in analysis response: {key}")

    @retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=4, max=10),
           retry=retry_if_exception_type((aiohttp.ClientError, asyncio.TimeoutError)))
    async def suggest_solutions(self, problem: str, num_solutions: int = 3) -> List[Dict[str, Any]]:
        cache_key = f"suggest_solutions:{problem}:{num_solutions}"
        if cache_key in self.cache:
            return self.cache[cache_key]

        prompt = self.prompt_generator.generate_solution_prompt(problem, num_solutions)
        endpoint = f"{self.base_url}/suggest"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        data = {"prompt": prompt}

        self.request_counter.labels(endpoint='suggest').inc()
        with self.request_latency.labels(endpoint='suggest').time():
            try:
                async with self.session.post(endpoint, headers=headers, json=data) as response:
                    response.raise_for_status()
                    result = await response.json()
                    solutions = self.prompt_generator.parse_solution_response(result.get("suggestions", ""))
                    self.cache[cache_key] = solutions
                    return solutions
            except (aiohttp.ClientError, asyncio.TimeoutError) as e:
                self.logger.error(f"API request failed: {e}")
                raise

    async def execute_tasks(self, tasks: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        async def execute_task(task):
            start_time = time.time()
            try:
                if task["type"] == "generate_code":
                    result = await self.generate_code(task["requirements"], task.get("language", "python"), task.get("code_style", "pep8"))
                elif task["type"] == "analyze_requirements":
                    result = await self.analyze_requirements(task["requirements"], task.get("categories"))
                elif task["type"] == "suggest_solutions":
                    result = await self.suggest_solutions(task["problem"], task.get("num_solutions", 3))
                else:
                    raise ValueError(f"Unknown task type: {task['type']}")
                return {"task_id": task["id"], "result": result, "status": "success", "execution_time": time.time() - start_time}
            except Exception as e:
                self.logger.error(f"Task execution failed: {e}")
                return {"task_id": task["id"], "result": None, "status": "failed", "error": str(e), "execution_time": time.time() - start_time}

        tasks_with_priority = [(task.get("priority", 0), i, task) for i, task in enumerate(tasks)]
        tasks_with_priority.sort(reverse=True)
        sorted_tasks = [task for _, _, task in tasks_with_priority]

        semaphore = asyncio.Semaphore(5)  # Limit concurrent tasks to 5
        async def execute_with_semaphore(task):
            async with semaphore:
                return await execute_task(task)

        results = await asyncio.gather(*[execute_with_semaphore(task) for task in sorted_tasks])
        return results

async def main():
    api_key = os.getenv("LLM_API_KEY", "<YOUR_API_KEY>")
    base_url = os.getenv("LLM_BASE_URL", "<YOUR_BASE_URL>")

    logging.basicConfig(level=logging.INFO)

    async with LLMClient(api_key, base_url) as client:
        code = await client.generate_code("Write a Python function to calculate fibonacci sequence")
        print(f"Generated code: {code}")

        analysis = await client.analyze_requirements("Create a web application that allows users to upload and share images")
        print(f"Requirement analysis: {json.dumps(analysis, indent=2)}")

        suggestions = await client.suggest_solutions("How to optimize database queries for large datasets?")
        print(f"Solution suggestions: {json.dumps(suggestions, indent=2)}")

        tasks = [
            {"id": 1, "type": "generate_code", "requirements": "Write a Python class for a simple calculator", "priority": 2},
            {"id": 2, "type": "analyze_requirements", "requirements": "Build a RESTful API for a todo list application", "priority": 1},
            {"id": 3, "type": "suggest_solutions", "problem": "What are the best practices for securing a Node.js application?", "priority": 3}
        ]
        results = await client.execute_tasks(tasks)
        print(f"Task execution results: {json.dumps(results, indent=2)}")

if __name__ == "__main__":
    asyncio.run(main())
```