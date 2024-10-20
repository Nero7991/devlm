# DevLM - Software Development with LLMs
### Oren's crude attempt at somewhat fully automating software development, testing and integration in his personal projects (Pre-Beta)

## Major Issues

- Dealbreaker: The needle in the haystack problem causes loops and repeated actions when a task has many steps. (Only tested with Claude Sonnet 3.5)

## Description

I'm one of the laziest person I know when it comes to completing personal projects that don't have a deadline or some overseer. I try to document my projects but get bored of documenting and they just sit in the dust once I've moved on to other ones which are destined again to the same fate. My gaol with DevLM was to have it complete/document the projects I never completed. I hope that a reasoning model with technical knowlegde related to my personal projects would solve this. Introduce, DevLM uses LLM and enables them to take actions. It provides project context so that LLMs can actually take the proper actions to complete a request. I've thrown in features to realise my vision for DevLM and it does do a pretty good job (it blew me away intially) but still fails at the vision. (Request it to add a feature or fix a bug and it does but not in the way you want it to)

The problem is that the LLM I'm using (Claude Sonnet 3.5) is not able to completely recall key events from the action history (context) (needle in the haystack). For example, when it runs an API process, it also receives the output for the process. When a test request is made using CURL, the server process output is provided to it. For outputs, in the recents actions (last 2), it is able to recall the output and take the right actions. But I'm giving it 15 actions and it is not able to recall the output for the older actions. This leads to it repeating certain actions and falling into a loop if the there are many steps to a particular user task/request. This is using just 50k context and claude supports 200k. What's the use of 200k if the LLM can't recall details in just ~50k?

I realise that models will improve over time (maybe I should try o1 preview or Opus 3.5 when its out) and once this context recall issue is solved, DevLM might be a very powerful tool to me.

**Warning**: This project is in a very early stage and may contain numerous issues and limitations.

## Actions LLM can take

The DevLM bootstrap script allows the LLM to take the following actions, for every action the LLM provides a reason, goal and chain of thought, each action is decided by the LLM based on the context provided (look below for what context is provided):

Example of an action by Claude:

```
<CoT>
Based on the user's last message to change the endpoint from /forgot-password to /forgot in the identity service, we need to modify the relevant files in the devlm-identity module. The most likely place for this change would be in the API specification and potentially in the auth or user implementation files. Let's start by inspecting the OpenAPI specification file and the main auth implementation file to locate the endpoint and make the necessary changes.
</CoT>

ACTION: INSPECT: devlm-identity/api/openapi.yaml, devlm-identity/internal/auth/auth.go

GOAL: Locate the /forgot-password endpoint in the OpenAPI specification and auth implementation to prepare for modification.

REASON: To change the endpoint as requested by the user, we first need to identify where it's defined in the API specification and potentially implemented in the code.
Response usage: Usage(input_tokens=17596, output_tokens=148)
Files analysis:
The inspection reveals that the /forgot-password endpoint is defined in the OpenAPI specification (devlm-identity/api/openapi.yaml) and implemented in the ForgotPassword function in the auth.go file. To change the endpoint to /forgot, we need to update both files. In the OpenAPI spec, modify the path from /api/v1/users/forgot-password to /api/v1/users/forgot. In auth.go, update the route registration in the RegisterRoutes function to use the new path. After making these changes, we should update any frontend code that uses this endpoint and then test the functionality to ensure it works correctly with the new path.
```
List of actions:

1. **RUN**: Execute a command synchronously from a predefined list of allowed commands. Output is provided after the command exits.
   Example: `RUN: python3 test_script.py`

2. **INDEF**: Run a command asynchronously (in the background). The output of the process is provided to the LLM in the next action.
   Example: `INDEF: python3 server.py`

3. **RAW**: Execute a raw shell command (requires user approval).
   Example: `RAW: ls -la`

4. **CHECK**: Check the output of a running process.
   Example: `CHECK: python3 server.py`

5. **INSPECT**: Analyze up to four files in the project structure. 
   Example: `INSPECT: file1.py, file2.py, file3.py, file4.py`

6. **MULTI**: Read multiple files and modify one of them.
   Example: `MULTI: file1.py, file2.py, file3.py; MODIFY: file2.py`

7. **CHAT**: Interact with the user to ask questions or provide feedback.
   Example: `CHAT: Should we implement feature X?`

8. **RESTART**: Restart a running process.
   Example: `RESTART: python3 server.py`

9. **DONE**: Indicate that testing is complete.

10. **UI Actions** (if frontend testing is enabled):
    - `UI_OPEN`: Open a URL in the browser.
    - `UI_CLICK`: Click a button on the webpage.
    - `UI_CHECK_TEXT`: Verify text content of an element.
    - `UI_CHECK_LOG`: Check console logs for specific messages.
    - `UI_XHR_CAPTURE_START`: Start capturing XHR requests.
    - `UI_XHR_CAPTURE_STOP`: Stop capturing XHR requests and get results.

These actions allow the LLM to interact with the project files, run commands, perform tests, and even conduct basic frontend testing. The exact behavior and availability of these actions may vary based on the mode (generate or test) and the specific configuration of the DevLM script.


## Context for Decision Making

The LLM receives the following context to decide its next action:

### Project Context
- Project Summary (from project_summary.md)
- Current project structure as a directory tree
- User notes from chat.txt
- Action history brief summarizing key events
- Last 20 actions taken and their results
- Status of currently running processes
- Latest process outputs
- Information about file modifications in the previous iteration

### Additional Context
- Whether the session just started
- Whether any code was modified in the previous iteration
- Current iteration number
- User suggestions (if any)
- Previous action analysis
- For UI testing: current URL and browser state

### Directives
The LLM follows specific directives for:
- Continuous development, testing, and debugging workflow
- Learning from previous interactions
- Code consistency and integration
- Environment handling
- Progress monitoring
- Process management
- Error resolution

The LLM uses this context to:
1. Evaluate the current state
2. Consider previous actions and their outcomes
3. Plan next steps based on project goals
4. Handle errors and unexpected situations
5. Maintain development progress




## Limitations and Precautions

- The LLM's ability to use these actions effectively depends on the quality of the project context provided and the specific prompts used.
- Some actions (like RAW commands) require user approval for safety reasons.
- The script has limited error handling, so unexpected inputs or outputs may cause issues.
- Frontend testing features are experimental and may not work consistently across all environments.


## Current Status

- Pre-beta stage
- May contain bugs and unexpected behavior
- Limited error handling and user guidance
- Functionality may change significantly in future versions

## Setup & Usage

1. Clone bootstrap script and dependencies:
```bash
git clone https://github.com/nero7991/devlm.git
cd devlm
pip install -r requirements.txt
```

2. Create a `project_summary.md` in your project's root directory describing the project's goals and requirements.

3. Run the script:
```bash
python3 bootstrap.py --mode [generate|test] --source [gcloud|anthropic] --project-path /path/to/your/project
```

Example:
```bash
python3 bootstrap.py --mode test --source gcloud --project-path ~/GitHub/myproject/
```

## File Organization

- `.devlm/` - Created in your project directory, contains:
  - Generated files
  - Command history
  - Technical briefs
  - Test progress
  - Project structure data

## Required Arguments

- `--mode`: 'generate' or 'test'
- `--source`: 'gcloud' or 'anthropic'
- `--project-path`: Full path to your project directory

## Optional Arguments

- `--frontend`: Enable UI testing
- `--api-key`: Anthropic API key (if using anthropic source)
- `--project-id`: Google Cloud project ID (if using gcloud source)
- `--region`: Google Cloud region (if using gcloud source)

## Known Limitations

- May produce inconsistent or incorrect code
- Limited understanding of complex project structures
- Potential for generating non-functional or syntactically incorrect code
- May not adhere to best practices or specific coding standards
- Frontend testing features are experimental and may not work as expected

## Contributing

As this project is in its early stages, we're primarily focusing on stabilizing core functionality. However, feedback and issue reports are welcome.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Disclaimer

DevLM is an experimental tool and should not be used for critical or production projects at this stage. Use at your own risk.