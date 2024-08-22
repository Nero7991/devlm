import re
from typing import List, Dict, Any

class PromptGenerator:
    def __init__(self, system_prompt: str = None):
        self.system_prompt = system_prompt or """You are an AI assistant specialized in software development. 
        Your task is to help with code generation, requirement analysis, and problem-solving."""
        self.languages = ['python', 'javascript', 'java', 'c++', 'go', 'rust', 'typescript', 'kotlin', 'swift']
        self.code_styles = {
            'python': ['PEP 8', 'Google Python Style Guide', 'Black'],
            'javascript': ['Airbnb JavaScript Style Guide', 'Google JavaScript Style Guide', 'StandardJS'],
            'java': ['Google Java Style Guide', 'Oracle Code Conventions', 'Android Code Style'],
            'c++': ['Google C++ Style Guide', 'LLVM Coding Standards', 'Mozilla C++ Coding Style'],
            'go': ['Effective Go', 'Uber Go Style Guide', 'Google Go Style Guide'],
            'rust': ['Rust Style Guide', 'Rustfmt'],
            'typescript': ['TSLint', 'Microsoft TypeScript Style Guide'],
            'kotlin': ['Kotlin Coding Conventions', 'Android Kotlin Style Guide'],
            'swift': ['Swift Style Guide', 'Apple Swift API Design Guidelines']
        }

    def generate_code_prompt(self, requirements: str, language: str, code_style: str = None) -> str:
        prompt = f"{self.system_prompt}\n\n"
        prompt += f"Generate code in {language} based on the following requirements:\n\n{requirements}\n\n"
        if code_style:
            prompt += f"Please follow the {code_style} coding style.\n"
        prompt += "Provide only the code without any additional explanations."
        prompt += "\nInclude detailed comments explaining the code's functionality and any important decisions made."
        prompt += "\nEnsure the code is efficient, readable, and follows best practices for the specified language."
        prompt += f"\nConsider language-specific features and idioms for {language}."
        prompt += "\nIf applicable, include error handling, input validation, and appropriate logging."
        prompt += "\nProvide any necessary setup instructions or dependencies as comments."
        return prompt

    def generate_analysis_prompt(self, requirements: str, categories: List[str] = None) -> str:
        prompt = f"{self.system_prompt}\n\n"
        prompt += f"Analyze the following project requirements:\n\n{requirements}\n\n"
        prompt += "Provide a structured analysis including:\n"
        categories = categories or ["Main features", "Potential challenges", "Suggested architecture", "Estimated timeline", "Technical considerations", "Security implications", "Scalability aspects", "Testing strategy", "Deployment considerations"]
        for i, category in enumerate(categories, 1):
            prompt += f"{i}. {category}\n"
        prompt += "\nFor each category, provide detailed explanations and, where applicable, examples or recommendations."
        prompt += "\nInclude any relevant industry standards or best practices in your analysis."
        prompt += "\nConsider potential risks and mitigation strategies for each aspect of the project."
        prompt += "\nProvide specific technology recommendations where appropriate."
        return prompt

    def generate_solution_prompt(self, problem: str, num_solutions: int = 3) -> str:
        prompt = f"{self.system_prompt}\n\n"
        prompt += f"Suggest {num_solutions} solutions for the following problem:\n\n{problem}\n\n"
        prompt += "For each solution, provide:\n"
        prompt += "1. A detailed explanation of the approach\n"
        prompt += "2. Pros and cons\n"
        prompt += "3. Implementation considerations\n"
        prompt += "4. Potential challenges and how to address them\n"
        prompt += "5. Estimated time and resources required\n"
        prompt += "6. Performance implications\n"
        prompt += "7. Scalability considerations\n"
        prompt += "8. Maintenance and long-term support aspects\n"
        prompt += "9. Cost considerations (if applicable)\n"
        prompt += "10. Compatibility with existing systems or technologies\n"
        return prompt

    def generate_task_prompts(self, tasks: List[Dict[str, Any]]) -> List[str]:
        prompts = []
        for task in tasks:
            task_type = task.get('type', '')
            task_details = task.get('details', '')
            
            try:
                if task_type == 'code_generation':
                    language = task.get('language', 'python').lower()
                    if language not in self.languages:
                        raise ValueError(f"Unsupported language: {language}")
                    code_style = task.get('code_style')
                    if code_style and code_style not in self.code_styles.get(language, []):
                        raise ValueError(f"Unsupported code style for {language}: {code_style}")
                    prompts.append(self.generate_code_prompt(task_details, language, code_style))
                elif task_type == 'requirement_analysis':
                    prompts.append(self.generate_analysis_prompt(task_details, task.get('categories')))
                elif task_type == 'problem_solving':
                    prompts.append(self.generate_solution_prompt(task_details, task.get('num_solutions', 3)))
                elif task_type == 'custom':
                    prompts.append(f"{self.system_prompt}\n\n{task_details}")
                else:
                    raise ValueError(f"Unrecognized task type: {task_type}")
            except Exception as e:
                prompts.append(f"Error generating prompt for task {task_type}: {str(e)}")
        
        return prompts

    def extract_code_from_response(self, response: str) -> Dict[str, str]:
        code_blocks = re.findall(r'```(\w*)\n(.*?)```', response, re.DOTALL)
        extracted_code = {}
        for language, code in code_blocks:
            lang = language.lower() or 'unknown'
            if lang not in self.languages:
                lang = self._detect_language(code)
            extracted_code[lang] = extracted_code.get(lang, '') + code.strip() + '\n'
        return extracted_code or {'unknown': response.strip()}

    def _detect_language(self, code: str) -> str:
        language_patterns = {
            'python': r'\bdef\b|\bimport\b|\bclass\b',
            'javascript': r'\bfunction\b|\bvar\b|\bconst\b|\blet\b',
            'java': r'\bpublic\b\s+\bclass\b|\bpublic\b\s+\bstatic\b\s+\bvoid\b\s+\bmain\b',
            'c++': r'\#include\b|\bint\b\s+\bmain\(\)|\bstd::|::|->',
            'go': r'\bpackage\b\s+\bmain\b|\bfunc\b\s+\bmain\(\)|\bfmt\.',
            'rust': r'\bfn\b\s+\bmain\(\)|\blet\b\s+mut\b|\buse\b\s+std::',
            'typescript': r'\binterface\b|\btype\b|\bnamespace\b',
            'kotlin': r'\bfun\b\s+\bmain\(|\bvalIMPORT\b|\bvar\b',
            'swift': r'\bimport\b\s+Foundation|\bclass\b|\bstruct\b|\blet\b|\bvar\b'
        }

        for lang, pattern in language_patterns.items():
            if re.search(pattern, code):
                return lang
        return 'unknown'

    def parse_analysis_response(self, response: str) -> Dict[str, Any]:
        sections = ['Main features', 'Potential challenges', 'Suggested architecture', 'Estimated timeline', 'Technical considerations', 'Security implications', 'Scalability aspects', 'Testing strategy', 'Deployment considerations']
        analysis = {}
        
        current_section = None
        lines = response.split('\n')
        for line in lines:
            line = line.strip()
            if line in sections:
                current_section = line.lower().replace(' ', '_')
                analysis[current_section] = []
            elif current_section and line:
                analysis[current_section].append(line)
        
        for section, content in analysis.items():
            subsections = {}
            current_subsection = None
            for item in content:
                if item.endswith(':'):
                    current_subsection = item[:-1].lower().replace(' ', '_')
                    subsections[current_subsection] = []
                elif current_subsection:
                    subsections[current_subsection].append(item)
                else:
                    subsections['main'] = subsections.get('main', []) + [item]
            analysis[section] = subsections
        
        return analysis

    def parse_solution_response(self, response: str) -> List[Dict[str, Any]]:
        solutions = []
        solution_blocks = re.split(r'\n(?=\d+\.)', response)
        
        for block in solution_blocks:
            if not block.strip():
                continue
            solution = {}
            lines = block.split('\n')
            current_section = 'explanation'
            for line in lines[1:]:  # Skip the solution number
                line = line.strip()
                if line.lower() in ['pros:', 'cons:', 'implementation considerations:', 'potential challenges:', 'estimated time and resources:', 'performance implications:', 'scalability considerations:', 'maintenance and long-term support aspects:', 'cost considerations:', 'compatibility with existing systems:']:
                    current_section = line.lower().replace(':', '').replace(' ', '_')
                    solution[current_section] = []
                elif line:
                    if current_section == 'explanation':
                        solution[current_section] = solution.get(current_section, '') + ' ' + line
                    else:
                        solution[current_section].append(line)
            
            solutions.append(solution)
        
        return solutions

def main():
    generator = PromptGenerator()
    
    code_prompt = generator.generate_code_prompt("Create a function to calculate the factorial of a number", "Python", "PEP 8")
    analysis_prompt = generator.generate_analysis_prompt("Build a web scraper that collects product information from e-commerce websites")
    solution_prompt = generator.generate_solution_prompt("How to optimize database queries for a high-traffic website?", 5)
    
    print("Code Generation Prompt:")
    print(code_prompt)
    print("\nRequirement Analysis Prompt:")
    print(analysis_prompt)
    print("\nProblem-Solving Prompt:")
    print(solution_prompt)
    
    code_response = """Here's a Python function to calculate factorial:
    ```python
    def factorial(n):
        if n == 0 or n == 1:
            return 1
        else:
            return n * factorial(n-1)
    ```
    """
    extracted_code = generator.extract_code_from_response(code_response)
    print("\nExtracted Code:")
    print(extracted_code)
    
    analysis_response = """
    Main features:
    - Web scraping functionality
    - Data extraction from e-commerce sites
    - Data storage mechanism
    
    Potential challenges:
    - Handling different website structures
    - Managing rate limiting and IP blocking
    - Ensuring data accuracy and consistency
    
    Suggested architecture:
    - Use Python with BeautifulSoup or Scrapy
    - Implement proxy rotation
    - Store data in a NoSQL database
    
    Estimated timeline:
    - 2-3 weeks for basic implementation
    - 1-2 weeks for testing and refinement
    
    Technical considerations:
    Performance optimization:
        - Implement concurrent scraping
        - Use asynchronous programming
    Error handling:
        - Implement robust error handling and logging
        - Develop a retry mechanism for failed requests
    
    Security implications:
    - Implement secure storage of scraped data
    - Use HTTPS for all requests
    - Handle and sanitize user inputs
    
    Scalability aspects:
    - Design for horizontal scaling
    - Implement a distributed scraping architecture
    - Use a message queue for task distribution
    """
    parsed_analysis = generator.parse_analysis_response(analysis_response)
    print("\nParsed Analysis:")
    print(parsed_analysis)
    
    solution_response = """
    1. Index optimization
    Explanation: Analyze query patterns and create appropriate indexes to improve query performance.
    Pros:
    - Significantly improves query speed
    - Reduces overall database load
    Cons:
    - Increases storage requirements
    - May impact write performance
    Implementation considerations:
    - Use database profiling tools to identify slow queries
    - Create composite indexes for frequently used query conditions
    Potential challenges:
    - Balancing index creation with write performance
    - Maintaining indexes as data and query patterns change
    Estimated time and resources:
    - 1-2 weeks for initial analysis and implementation
    - Ongoing monitoring and optimization
    Performance implications:
    - Faster read operations
    - Slightly slower write operations
    Scalability considerations:
    - Indexes may need to be adjusted as data volume grows
    - Consider partitioning for very large datasets
    Maintenance and long-term support aspects:
    - Regular index usage analysis
    - Periodic index reorganization or rebuilding
    Cost considerations:
    - Increased storage costs for indexes
    - Potential need for more powerful database servers
    Compatibility with existing systems:
    - Ensure compatibility with current ORM or query builders
    - May require updates to existing queries for optimal performance

    2. Query caching
    Explanation: Implement a caching layer to store frequently accessed query results.
    Pros:
    - Drastically reduces database load for repetitive queries
    - Improves response times for cached queries
    Cons:
    - Increases system complexity
    - Requires careful cache invalidation strategies
    Implementation considerations:
    - Choose an appropriate caching solution (e.g., Redis, Memcached)
    - Implement intelligent cache expiration and invalidation mechanisms
    Potential challenges:
    - Ensuring cache consistency with the database
    - Handling cache warm-up after system restarts
    Estimated time and resources:
    - 2-3 weeks for implementation and testing
    - Ongoing maintenance and fine-tuning
    Performance implications:
    - Significantly faster response times for cached queries
    - Reduced database load
    Scalability considerations:
    - Cache can be distributed across multiple nodes
    - May need to implement cache sharding for very large datasets
    Maintenance and long-term support aspects:
    - Monitoring cache hit/miss ratios
    - Adjusting cache policies based on usage patterns
    Cost considerations:
    - Additional infrastructure costs for caching servers
    - Potential savings in database resources
    Compatibility with existing systems:
    - Ensure compatibility with current application architecture
    - May require modifications to existing data access layers
    """
    parsed_solutions = generator.parse_solution_response(solution_response)
    print("\nParsed Solutions:")
    print(parsed_solutions)

if __name__ == "__main__":
    main()