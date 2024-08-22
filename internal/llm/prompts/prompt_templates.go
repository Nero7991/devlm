package prompts

import (
	"text/template"
)

type PromptTemplates struct {
	InitialPrompt       *template.Template
	CodeGeneration      *template.Template
	CodeReview          *template.Template
	ErrorAnalysis       *template.Template
	WebSearch           *template.Template
	FileOperation       *template.Template
	TestGeneration      *template.Template
	RequirementAnalysis *template.Template
	TaskDecomposition   *template.Template
	ProgressUpdate      *template.Template
}

func NewPromptTemplates() (*PromptTemplates, error) {
	templates := &PromptTemplates{}
	var err error

	// Initialize templates
	templateDefinitions := map[string]string{
		"initial": `Given the following requirements:
{{.Requirements}}
Please analyze them and provide an initial plan for development. Consider the overall architecture, main components, and potential challenges. Include:
1. A high-level system architecture diagram (in text format)
2. Main components and their responsibilities
3. Potential technical challenges and proposed solutions
4. Suggested tech stack with justifications
5. Initial project timeline with major milestones`,

		"codeGen": `Context: {{.Context}}
Task: {{.Task}}
Language: {{.Language}}
Framework: {{.Framework}}
Additional Requirements:
{{range .AdditionalRequirements}}
- {{.}}
{{end}}

Please generate code to accomplish this task. Your response should include:
1. Complete, well-structured code that follows best practices for the specified language and framework
2. Inline comments explaining complex logic or design decisions
3. A brief explanation of the code structure and any notable design patterns used
4. Suggestions for potential optimizations or alternative approaches
5. Any assumptions made during the implementation

Ensure the code is efficient, maintainable, and adheres to common coding standards for {{.Language}}.`,

		"codeReview": `Code:
{{.Code}}

Requirements:
{{.Requirements}}

Focus Areas: {{.FocusAreas}}

Please conduct a comprehensive code review, focusing on the specified areas. Your review should include:

1. Functionality: Does the code correctly implement the requirements?
2. Efficiency: Are there any performance bottlenecks or areas for optimization?
3. Maintainability: Is the code well-structured, readable, and easy to maintain?
4. Best Practices: Does the code follow language-specific and general coding best practices?
5. Error Handling: Is error handling comprehensive and appropriate?
6. Security: Are there any potential security vulnerabilities?
7. Testing: Is the code testable, and are there suggestions for unit tests?

For each identified issue or suggestion:
- Provide a clear explanation of the problem or improvement opportunity
- Suggest a specific code change or approach to address the issue
- Explain the rationale behind your suggestion

Please prioritize your feedback, focusing on the most critical issues first.`,

		"errorAnalysis": `Code:
{{.Code}}

Error:
{{.Error}}

Error Category: {{.Category}}

Please provide a detailed analysis of this {{.Category}} error. Your response should include:

1. Root Cause Analysis:
   - Explain the fundamental reason for this error
   - Identify any contributing factors in the code or environment

2. Impact Assessment:
   - Describe the potential impact of this error on the system
   - Highlight any security or performance implications

3. Detailed Fix:
   - Provide a step-by-step solution to resolve the error
   - Include code examples demonstrating the fix
   - Explain any trade-offs or considerations in your proposed solution

4. Prevention Strategies:
   - Suggest best practices or coding patterns to prevent similar errors in the future
   - Recommend any tools or techniques that could help catch this type of error earlier

5. Related Concepts:
   - Briefly explain any related programming concepts or common pitfalls associated with this error

Please ensure your explanation is clear and actionable, suitable for developers of varying experience levels.`,

		"webSearch": `Please perform a web search for the following query:
{{.Query}}

Preferred Sources: {{.PreferredSources}}
Exclude Domains: {{.ExcludeDomains}}

In your search results, please provide:

1. A summary of the most relevant and reliable information found
2. Direct quotes from authoritative sources, with proper citations
3. Links to the top 3-5 most relevant web pages
4. Any conflicting information or debates found on the topic
5. Suggestions for related queries that might provide additional useful information

Ensure that the information is up-to-date and comes from reputable sources. If the search query is related to code or programming, include code snippets or documentation links where applicable.`,

		"fileOp": `Operation: {{.Operation}}
Path: {{.Path}}
Content: {{.Content}}
Permissions: {{.Permissions}}

Please perform this file operation and provide a detailed response including:

1. A step-by-step explanation of the operation being performed
2. Potential errors that could occur (e.g., file not found, permission issues) and how to handle them
3. Best practices for this type of file operation in the context of a larger application
4. Any security considerations related to this file operation
5. Suggestions for error logging and reporting
6. If applicable, code snippets demonstrating how to perform this operation safely and efficiently

For write operations, include checksum verification steps. For read operations, suggest efficient methods for processing large files. Always prioritize data integrity and security in your approach.`,

		"testGen": `Code:
{{.Code}}

Requirements:
{{.Requirements}}

Test Framework: {{.TestFramework}}

Please generate comprehensive unit tests for this code using the specified test framework. Your response should include:

1. A complete set of unit tests covering all public functions and methods
2. Tests for both expected behavior and edge cases
3. Error scenario tests to ensure proper error handling
4. Mocking of external dependencies where appropriate
5. Clear test case descriptions and assertions
6. Setup and teardown procedures if necessary
7. Suggestions for integration or end-to-end tests, if applicable

Ensure that the tests follow best practices for {{.TestFramework}} and provide good code coverage. Include comments explaining the rationale behind complex test scenarios.`,

		"reqAnalysis": `Requirements:
{{.Requirements}}

Please provide a comprehensive analysis of these requirements. Your response should include:

1. Categorization of requirements:
   - Functional requirements
   - Non-functional requirements (performance, security, scalability, etc.)
   - Constraints and limitations

2. Breakdown of requirements into specific, actionable tasks:
   - Prioritized list of tasks
   - Estimated complexity for each task (e.g., low, medium, high)
   - Dependencies between tasks

3. Identification of potential conflicts or ambiguities:
   - List any requirements that may conflict with each other
   - Highlight areas where more clarification is needed

4. Suggested clarifying questions:
   - Provide a list of questions to ask stakeholders for any unclear points

5. Initial architecture considerations:
   - High-level system components based on the requirements
   - Potential technical challenges and proposed solutions

6. Risk assessment:
   - Identify potential risks or challenges in implementing these requirements
   - Suggest mitigation strategies for each identified risk

7. Acceptance criteria:
   - Draft measurable acceptance criteria for key requirements

Please ensure your analysis is thorough and actionable, providing a solid foundation for the development process.`,

		"taskDecomp": `Task:
{{.Task}}

Desired Complexity Level: {{.ComplexityLevel}}

Please decompose this task into smaller, manageable subtasks. Your response should include:

1. A hierarchical breakdown of the main task into subtasks:
   - Aim for a {{.ComplexityLevel}} level of granularity
   - Ensure each subtask is specific and actionable

2. For each subtask:
   - Provide a clear description
   - Estimate the complexity (e.g., low, medium, high)
   - Identify any dependencies on other subtasks

3. A suggested logical order for tackling these subtasks:
   - Consider dependencies and optimal workflow

4. Potential challenges or risks associated with each subtask:
   - Brief description of the challenge
   - Suggested mitigation strategies

5. Resources or skills required for each subtask:
   - Technical skills needed
   - Any specific tools or technologies required

6. Estimated time range for each subtask:
   - Provide a rough estimate (e.g., 2-4 hours, 1-2 days)

7. Suggestions for parallel work:
   - Identify subtasks that could be worked on simultaneously

Please ensure the decomposition is comprehensive and aligns with the desired complexity level, providing a clear roadmap for implementing the main task.`,

		"progressUpdate": `Completed Tasks:
{{range .CompletedTasks}}
- {{.Task}} (Time Taken: {{.TimeTaken}}, Priority: {{.Priority}})
{{end}}

Remaining Tasks:
{{range .RemainingTasks}}
- {{.Task}} (Estimated Time: {{.EstimatedTime}}, Priority: {{.Priority}})
{{end}}

Please provide a comprehensive progress update. Your response should include:

1. Summary of Progress:
   - Brief overview of what has been accomplished
   - Comparison of actual progress against initial estimates

2. Detailed Analysis of Completed Tasks:
   - Successes and challenges encountered
   - Lessons learned and best practices identified

3. Blockers and Risks:
   - Description of any current blockers
   - Potential risks identified for remaining tasks
   - Suggested mitigation strategies

4. Next Steps:
   - Prioritized list of upcoming tasks
   - Focus on high-priority items
   - Any adjustments needed to the original plan

5. Resource Allocation:
   - Current team workload
   - Suggestions for optimizing resource allocation

6. Timeline Update:
   - Revised estimates for project completion
   - Explanation of any changes to the original timeline

7. Quality Assurance:
   - Overview of testing and code review status
   - Any quality concerns and proposed solutions

8. Stakeholder Communication:
   - Key points to communicate to stakeholders
   - Any decisions or input needed from management

Please provide a clear and actionable update that gives a comprehensive view of the project status and guides the team towards successful completion.`,
	}

	for name, content := range templateDefinitions {
		t, err := template.New(name).Parse(content)
		if err != nil {
			return nil, err
		}
		switch name {
		case "initial":
			templates.InitialPrompt = t
		case "codeGen":
			templates.CodeGeneration = t
		case "codeReview":
			templates.CodeReview = t
		case "errorAnalysis":
			templates.ErrorAnalysis = t
		case "webSearch":
			templates.WebSearch = t
		case "fileOp":
			templates.FileOperation = t
		case "testGen":
			templates.TestGeneration = t
		case "reqAnalysis":
			templates.RequirementAnalysis = t
		case "taskDecomp":
			templates.TaskDecomposition = t
		case "progressUpdate":
			templates.ProgressUpdate = t
		}
	}

	return templates, nil
}