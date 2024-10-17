# DevLM - Software Development with LLMs
### My crude attempt as somewhat fully automating software development, testing and integration in my personal projects (Pre-Beta)

## Description

I'm one of the laziest person I know when it comes to personal projects that don't have a deadline or some overseer leading to me never finishing them. I try to document my projects but hate documenting and they just sit in the dust once I've moved on to other ones which are destined again to the same fate. My gaol with DevLM was to have it complete/document the projects I never completed. I hope that a reasoning model with technical knowlegde related to my personal projects would solve this. Introduce, DevLM uses LLM and enables them to take actions. It provides project context so that LLMs can actually take the proper actions to complete a request. I've thrown in features to realise my vision for DevLM and it does do a pretty good job (it blew me away intially) but still fails at the vision. (Request it to add a feature or fix a bug and it does but not in the way you want it to)

The problem is that the LLM I'm using (Claude Sonnet 3.5) is not able to completely recall key events from the action history (context). For example, when it runs an API process, it also receives the output for the process. When a test request is made using CURL, the server process output is provided to it. For outputs, in the recents actions (last 2), it is able to recall the output and take the right actions. But I'm giving it 15 actions and it is not able to recall the output for the older actions. This leads to it repeating certain actions and falling into a loop if the there are many steps to a particular user task/request. This is using just 50k context and claude supports 200k. What's the use of 200k if the LLM can't recall details in just ~50k?

I realise that models will improve over time (maybe I should try o1 preview or Opus 3.5 when its out) and once this context recall issue is solved, DevLM might be a very powerful tool to me.

**Warning**: This project is in a very early stage and may contain numerous issues and limitations.

## LLM Actions

The DevLM bootstrap script allows the LLM to take the following actions:

1. **RUN**: Execute a command synchronously from a predefined list of allowed commands. 
   Example: `RUN: python3 test_script.py`

2. **INDEF**: Run a command asynchronously (in the background).
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

## Basic Setup

1. Copy the `bootstrap.py` script to the root directory of your project.

2. Create a `project_summary.md` file in the same directory. This file should contain a description of your project, providing context for the LLM. This need to be automated by using the LLM write it.

3. Ensure you have the necessary dependencies installed (see Prerequisites section).

## Prerequisites

- Python 3.7+
- Required Python packages:
  - anthropic
  - google-auth
  - google-auth-oauthlib
  - google-auth-httplib2
  - google-cloud-aiplatform
  - selenium (for frontend testing)
  - webdriver_manager (for frontend testing)

Install dependencies with:
```
pip install anthropic google-auth google-auth-oauthlib google-auth-httplib2 google-cloud-aiplatform selenium webdriver_manager
```

## Basic Usage

Run the script in your project directory:

```
python bootstrap.py --mode [generate|test] [options]
```

Main options:
- `--mode`: Choose 'generate' for code generation or 'test' for interactive testing.
- `--source`: Select 'gcloud' or 'anthropic' for the LLM source.

For a full list of options, run:
```
python bootstrap.py --help
```

## Configuration

- `project_summary.md`: Crucial for providing project context to the LLM.
- `project_structure.json`: Will be generated if not present, defines your project structure.
- Environment variables: Set up in `devlm.env` file (API keys, project IDs, etc.)

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