import unittest
from llm_service.prompt_generator import PromptGenerator

class TestPromptGenerator(unittest.TestCase):

    def setUp(self):
        self.prompt_generator = PromptGenerator()

    def test_generate_code_prompt(self):
        requirements = "Create a function to calculate the factorial of a number"
        expected_prompt = "Generate Python code for the following requirement:\nCreate a function to calculate the factorial of a number\n\nPlease provide only the code without any explanations."
        
        result = self.prompt_generator.generate_code_prompt(requirements)
        self.assertEqual(result, expected_prompt)

    def test_generate_analysis_prompt(self):
        requirements = "Build a web scraper for e-commerce websites"
        expected_prompt = "Analyze the following project requirement and provide a detailed breakdown:\nBuild a web scraper for e-commerce websites\n\nPlease include potential challenges, necessary components, and suggested implementation steps."
        
        result = self.prompt_generator.generate_analysis_prompt(requirements)
        self.assertEqual(result, expected_prompt)

    def test_generate_improvement_prompt(self):
        code = "def fibonacci(n):\n    if n <= 1:\n        return n\n    else:\n        return fibonacci(n-1) + fibonacci(n-2)"
        expected_prompt = "Suggest improvements for the following code:\n\ndef fibonacci(n):\n    if n <= 1:\n        return n\n    else:\n        return fibonacci(n-1) + fibonacci(n-2)\n\nPlease provide specific suggestions to enhance performance, readability, or functionality."
        
        result = self.prompt_generator.generate_improvement_prompt(code)
        self.assertEqual(result, expected_prompt)

    def test_generate_explanation_prompt(self):
        code = "def quicksort(arr):\n    if len(arr) <= 1:\n        return arr\n    pivot = arr[len(arr) // 2]\n    left = [x for x in arr if x < pivot]\n    middle = [x for x in arr if x == pivot]\n    right = [x for x in arr if x > pivot]\n    return quicksort(left) + middle + quicksort(right)"
        expected_prompt = "Explain the following code in detail:\n\ndef quicksort(arr):\n    if len(arr) <= 1:\n        return arr\n    pivot = arr[len(arr) // 2]\n    left = [x for x in arr if x < pivot]\n    middle = [x for x in arr if x == pivot]\n    right = [x for x in arr if x > pivot]\n    return quicksort(left) + middle + quicksort(right)\n\nPlease provide a step-by-step explanation of how this code works."
        
        result = self.prompt_generator.generate_explanation_prompt(code)
        self.assertEqual(result, expected_prompt)

    def test_generate_code_prompt_empty_input(self):
        requirements = ""
        expected_prompt = "Generate Python code for the following requirement:\n\n\nPlease provide only the code without any explanations."
        
        result = self.prompt_generator.generate_code_prompt(requirements)
        self.assertEqual(result, expected_prompt)

    def test_generate_analysis_prompt_multiline_input(self):
        requirements = "1. Create a user authentication system\n2. Implement a database to store user information\n3. Design a responsive UI for mobile and desktop"
        expected_prompt = "Analyze the following project requirement and provide a detailed breakdown:\n1. Create a user authentication system\n2. Implement a database to store user information\n3. Design a responsive UI for mobile and desktop\n\nPlease include potential challenges, necessary components, and suggested implementation steps."
        
        result = self.prompt_generator.generate_analysis_prompt(requirements)
        self.assertEqual(result, expected_prompt)

    def test_generate_improvement_prompt_complex_code(self):
        code = """
def bubble_sort(arr):
    n = len(arr)
    for i in range(n):
        for j in range(0, n - i - 1):
            if arr[j] > arr[j + 1]:
                arr[j], arr[j + 1] = arr[j + 1], arr[j]
    return arr
        """
        expected_prompt = f"Suggest improvements for the following code:\n\n{code}\n\nPlease provide specific suggestions to enhance performance, readability, or functionality."
        
        result = self.prompt_generator.generate_improvement_prompt(code)
        self.assertEqual(result, expected_prompt)

    def test_generate_explanation_prompt_with_comments(self):
        code = """
# This function calculates the nth Fibonacci number
def fibonacci(n):
    if n <= 1:
        return n
    else:
        return fibonacci(n-1) + fibonacci(n-2)
        """
        expected_prompt = f"Explain the following code in detail:\n\n{code}\n\nPlease provide a step-by-step explanation of how this code works."
        
        result = self.prompt_generator.generate_explanation_prompt(code)
        self.assertEqual(result, expected_prompt)

    def test_generate_code_prompt_with_language_specification(self):
        requirements = "Create a function to reverse a string"
        language = "JavaScript"
        expected_prompt = f"Generate {language} code for the following requirement:\nCreate a function to reverse a string\n\nPlease provide only the code without any explanations."
        
        result = self.prompt_generator.generate_code_prompt(requirements, language)
        self.assertEqual(result, expected_prompt)

    def test_generate_analysis_prompt_with_context(self):
        requirements = "Implement a caching system for database queries"
        context = "The application is a high-traffic e-commerce platform"
        expected_prompt = f"Analyze the following project requirement and provide a detailed breakdown:\nImplement a caching system for database queries\n\nContext: {context}\n\nPlease include potential challenges, necessary components, and suggested implementation steps."
        
        result = self.prompt_generator.generate_analysis_prompt(requirements, context)
        self.assertEqual(result, expected_prompt)

    def test_generate_improvement_prompt_with_specific_focus(self):
        code = """
def calculate_average(numbers):
    total = 0
    for num in numbers:
        total += num
    return total / len(numbers)
        """
        focus = "memory efficiency"
        expected_prompt = f"Suggest improvements for the following code, focusing on {focus}:\n\n{code}\n\nPlease provide specific suggestions to enhance {focus}."
        
        result = self.prompt_generator.generate_improvement_prompt(code, focus)
        self.assertEqual(result, expected_prompt)

    def test_generate_explanation_prompt_with_target_audience(self):
        code = """
def binary_search(arr, target):
    left, right = 0, len(arr) - 1
    while left <= right:
        mid = (left + right) // 2
        if arr[mid] == target:
            return mid
        elif arr[mid] < target:
            left = mid + 1
        else:
            right = mid - 1
    return -1
        """
        audience = "beginner programmers"
        expected_prompt = f"Explain the following code in detail for {audience}:\n\n{code}\n\nPlease provide a step-by-step explanation of how this code works, suitable for {audience}."
        
        result = self.prompt_generator.generate_explanation_prompt(code, audience)
        self.assertEqual(result, expected_prompt)

    def test_generate_code_prompt_with_multiple_languages(self):
        requirements = "Implement a binary search tree"
        languages = ["Python", "Java", "C++"]
        for lang in languages:
            expected_prompt = f"Generate {lang} code for the following requirement:\nImplement a binary search tree\n\nPlease provide only the code without any explanations."
            result = self.prompt_generator.generate_code_prompt(requirements, lang)
            self.assertEqual(result, expected_prompt)

    def test_generate_analysis_prompt_with_project_constraints(self):
        requirements = "Develop a real-time chat application"
        constraints = "Must be scalable to 1 million concurrent users and use WebSockets"
        expected_prompt = f"Analyze the following project requirement and provide a detailed breakdown:\nDevelop a real-time chat application\n\nConstraints: {constraints}\n\nPlease include potential challenges, necessary components, and suggested implementation steps, considering the given constraints."
        
        result = self.prompt_generator.generate_analysis_prompt(requirements, constraints=constraints)
        self.assertEqual(result, expected_prompt)

    def test_generate_improvement_prompt_with_multiple_aspects(self):
        code = """
def factorial(n):
    if n == 0:
        return 1
    else:
        return n * factorial(n - 1)
        """
        aspects = ["performance", "readability", "error handling"]
        expected_prompt = f"Suggest improvements for the following code, focusing on {', '.join(aspects)}:\n\n{code}\n\nPlease provide specific suggestions to enhance each aspect: {', '.join(aspects)}."
        
        result = self.prompt_generator.generate_improvement_prompt(code, aspects=aspects)
        self.assertEqual(result, expected_prompt)

    def test_generate_explanation_prompt_with_code_snippet_and_full_file(self):
        code_snippet = "result = [x for x in range(10) if x % 2 == 0]"
        full_file = """
def process_data(data):
    # Some processing logic here
    pass

result = [x for x in range(10) if x % 2 == 0]

def main():
    data = load_data()
    process_data(data)
    print(result)

if __name__ == "__main__":
    main()
        """
        expected_prompt = f"Explain the following code snippet in detail:\n\n{code_snippet}\n\nThis snippet is part of the following complete file:\n\n{full_file}\n\nPlease provide a step-by-step explanation of how the code snippet works and how it fits into the context of the entire file."
        
        result = self.prompt_generator.generate_explanation_prompt(code_snippet, full_file=full_file)
        self.assertEqual(result, expected_prompt)

    def test_generate_code_prompt_with_design_pattern(self):
        requirements = "Implement a logging system"
        design_pattern = "Singleton"
        expected_prompt = f"Generate Python code for the following requirement using the {design_pattern} design pattern:\nImplement a logging system\n\nPlease provide only the code without any explanations."
        
        result = self.prompt_generator.generate_code_prompt(requirements, design_pattern=design_pattern)
        self.assertEqual(result, expected_prompt)

    def test_generate_analysis_prompt_with_performance_requirements(self):
        requirements = "Create a sorting algorithm"
        performance_req = "Must have O(n log n) average time complexity"
        expected_prompt = f"Analyze the following project requirement and provide a detailed breakdown:\nCreate a sorting algorithm\n\nPerformance requirement: {performance_req}\n\nPlease include potential challenges, necessary components, and suggested implementation steps, considering the given performance requirement."
        
        result = self.prompt_generator.generate_analysis_prompt(requirements, performance_req=performance_req)
        self.assertEqual(result, expected_prompt)

    def test_generate_improvement_prompt_with_specific_language_version(self):
        code = """
async def fetch_data(url):
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            return await response.json()
        """
        language_version = "Python 3.9+"
        expected_prompt = f"Suggest improvements for the following code, considering features available in {language_version}:\n\n{code}\n\nPlease provide specific suggestions to enhance performance, readability, or functionality using {language_version} features."
        
        result = self.prompt_generator.generate_improvement_prompt(code, language_version=language_version)
        self.assertEqual(result, expected_prompt)

    def test_generate_explanation_prompt_with_specific_concepts(self):
        code = """
class MyContextManager:
    def __enter__(self):
        print("Entering the context")
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        print("Exiting the context")
        """
        concepts = ["context managers", "magic methods"]
        expected_prompt = f"Explain the following code in detail, focusing on the concepts of {', '.join(concepts)}:\n\n{code}\n\nPlease provide a step-by-step explanation of how this code works, emphasizing the role of {' and '.join(concepts)}."
        
        result = self.prompt_generator.generate_explanation_prompt(code, concepts=concepts)
        self.assertEqual(result, expected_prompt)

if __name__ == '__main__':
    unittest.main()