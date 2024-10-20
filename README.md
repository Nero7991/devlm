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
```

You can either create a virtual environment or install the dependencies in your current environment.
For virtual environment, use the following command:
```bash
python3 -m venv devlm-env
source devlm-env/bin/activate
pip3 install -r requirements.txt
```

Or install the dependencies in your current environment:
```bash
pip3 install -r requirements.txt
```

2. Create a `project_summary.md` in your project's root directory describing the project's goals and requirements.

3. Run the script:
```bash
python3 bootstrap.py --mode [generate|test] --source [gcloud|anthropic] --project-path /path/to/your/project
```

There are two modes:
- `generate`: The LLM will generate a project directory structure based on the project summary and create empty files. Originally, plan was to generate the initial code in generate mode as well but it was too unreliable. Test mode does that much better.
- `test`: Develop, test and debug mode. Chat with the LLM to develop your project.

### Example of develop, test and debug mode:
```
(myenv) orencollaco@Oren-Dell-Ubuntu:~/GitHub/devlm$ python3 bootstrap.py --mode test --source gcloud  --project-path ~/GitHub/devlm/devlm-identity/
Using Claude via Google Cloud. Project ID: devlm-434202, Region: us-east5
Working directory set to: /home/orencollaco/GitHub/devlm/devlm-identity/
Project directory structure has been generated.
Created new command history file: .devlm/command_history_20241019_232712.json
Warning: project_summary.md file not found. This file is important for providing context to the LLM.
For the current session, project summary to the LLM will be provided as 'No project summary found'
Entering test and debug mode...
History brief: {}
What would you like to accomplish in this session? Test the /registration endpoint using CURL by creating a user and then exit using DONE action
Running processes for debug: []
Process outputs for debug: []

Generating next step (Iteration 1)...
Response usage: Usage(input_tokens=1559, output_tokens=150)
LLM response:
<CoT>
To test the /registration endpoint using CURL, we need to:
1. Ensure the API server is running
2. Craft a CURL command to send a POST request to the /registration endpoint
3. Execute the CURL command and analyze the response

First, let's check if the API server is running. If not, we'll need to start it.
</CoT>

ACTION: INDEF: go run cmd/api/main.go
GOAL: Start the API server to test the /registration endpoint
REASON: We need a running server to test the endpoint. Starting it in the background allows us to interact with it using CURL.

Executing command: INDEF: go run cmd/api/main.go
Process Info: {'cmd': 'go run cmd/api/main.go', 'process': <Popen: returncode: None args: ['go', 'run', 'cmd/api/main.go']>, 'queue': <queue.Queue object at 0x78959f5853f0>, 'cwd': '/home/orencollaco/GitHub/devlm/devlm-identity', 'run_command': 'go run cmd/api/main.go', 'pid': 54444, 'child_pids': [54590]}
Command output:
Started new process: go run cmd/api/main.go
Initial PID: 54444
Child PIDs: [54590]
Initial output:
ly
PostgresDB: 2024/10/19 23:27:59 postgres.go:535: Database schema check passed successfully
devlm-identity: 2024/10/19 23:27:59 main.go:56: Database schema check passed
devlm-identity: 2024/10/19 23:27:59 main.go:116: Debug - Registered routes:
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/register [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/login [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/forgot-password [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/reset-password [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/refresh-token [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/profile [GET]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/change-password [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users [GET]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/{id} [GET]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/{id} [PUT]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/{id} [DELETE]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/{id}/update-role [PATCH]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/users/{id}/role [GET]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/auth/assign-role [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/auth/ssh-keys [GET]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/auth/ssh-keys [POST]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: /api/v1/auth/ssh-keys/{id} [DELETE]
devlm-identity: 2024/10/19 23:27:59 main.go:120: Debug - Route: / []
devlm-identity: 2024/10/19 23:27:59 main.go:133: Starting server on :8080

Running processes for debug: ['go run cmd/api/main.go']
Process outputs for debug: []
.
.
.
Generating next step (Iteration 7)...
Response usage: Usage(input_tokens=11329, output_tokens=288)
LLM response:
<CoT>
Given that we've successfully registered a new user, the next logical step is to test the login functionality with the newly created user credentials. This will ensure that the registration process was complete and that the user can access the system. We should use the curl command to send a POST request to the login endpoint with the credentials we just created.

I've considered the following:
1. We've completed the registration process successfully.
2. The next step in user authentication flow is typically login.
3. Testing the login will verify that the user was properly created and stored in the database.
4. This action aligns with the directive to test related functionalities after a successful operation.
5. We haven't tested the login functionality yet, so this is a new and necessary step.
</CoT>

ACTION: RUN: curl -X POST -H "Content-Type: application/json" -d '{"username":"uniqueuser","password":"Password123!"}' http://localhost:8080/api/v1/users/login

GOAL: Verify that the newly registered user can successfully log in to the system.

REASON: After successfully registering a new user, it's crucial to test the login functionality to ensure the user was properly created and can access the system. This step validates the entire registration and authentication process.
Checking environment...
Current directory: /home/orencollaco/GitHub/devlm/devlm-identity
Curl version: Command: curl --version
STDOUT:
curl 7.81.0 (x86_64-pc-linux-gnu) libcurl/7.81.0 OpenSSL/3.0.2 zlib/1.2.11 brotli/1.0.9 zstd/1.4.8 libidn2/2.3.2 libpsl/0.21.0 (+libidn2/2.3.2) libssh/0.9.6/openssl/zlib nghttp2/1.43.0 librtmp/2.3 OpenLDAP/2.5.18
Release-Date: 2022-01-05
Protocols: dict file ftp ftps gopher gophers http https imap imaps ldap ldaps mqtt pop3 pop3s rtmp rtsp scp sftp smb smbs smtp smtps telnet tftp 
Features: alt-svc AsynchDNS brotli GSS-API HSTS HTTP2 HTTPS-proxy IDN IPv6 Kerberos Largefile libz NTLM NTLM_WB PSL SPNEGO SSL TLS-SRP UnixSockets zstd

STDERR:

Executing command: curl -X POST -H "Content-Type: application/json" -d '{"username":"uniqueuser","password":"Password123!"}' http://localhost:8080/api/v1/users/login
Command output:
Command: curl -X POST -H "Content-Type: application/json" -d '{"username":"uniqueuser","password":"Password123!"}' http://localhost:8080/api/v1/users/login
STDOUT:
{"refresh_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzAwMDY5MzUsInVzZXJfaWQiOiI4Yjk3YzBkMS0wMTU5LTQ5MzctYTIyMy0yZDJjNjE0NTVlMDYifQ.9qTIv4k6EuOb3u6Y4vcitfL8KpyYlPAImhd8ixOfpUQ","token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3Mjk0ODg1MzUsInJvbGUiOiJ1c2VyIiwidXNlcl9pZCI6IjhiOTdjMGQxLTAxNTktNDkzNy1hMjIzLTJkMmM2MTQ1NWUwNiIsInVzZXJuYW1lIjoidW5pcXVldXNlciJ9.a2xC0LJt4V-GK6Y9ZwyXPR59vCexxAPNuit7zvxd2wk"}

STDERR:
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed

  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
100   475  100   424  100    51   8034    966 --:--:-- --:--:-- --:--:--  9134
Response usage: Usage(input_tokens=12096, output_tokens=189)
Command analysis:
The login attempt for the newly created user "uniqueuser" was successful. The server responded with a JSON object containing two tokens:

1. A refresh token for obtaining new access tokens.
2. An access token (labeled as "token") for authenticating API requests.

The presence of these tokens indicates that the user's credentials were validated correctly, and the authentication system is working as expected. The access token includes claims for the user's role ("user"), user ID, and username, which is standard for JWT-based authentication systems.

Next steps:
1. Test protected endpoints using the access token to ensure proper authorization.
2. Verify token expiration and refresh functionality.
3. Test invalid login attempts to confirm proper error handling.
4. Consider implementing additional security measures like rate limiting for login attempts.

No debugging steps are necessary as the login process completed successfully without any errors.
Running processes for debug: ['go run cmd/api/main.go']
Process outputs for debug: ['Latest output from \'go run cmd/api/main.go\':\n2024/10/19 23:28:55 BodyParserMiddleware: Entering for request: POST /api/v1/users/login\n2024/10/19 23:28:55 BodyParserMiddleware: Raw request body: {"username":"uniqueuser","password":"Password123!"}\n2024/10/19 23:28:55 BodyParserMiddleware: Parsed JSON body: map[password:Password123! username:uniqueuser]\n2024/10/19 23:28:55 BodyParserMiddleware: Context values after processing: context.Background.WithValue(type *http.contextKey, val <not Stringer>).WithValue(type *http.contextKey, val 127.0.0.1:8080).WithCancel.WithCancel.WithValue(type mux.contextKey, val <not Stringer>).WithValue(type mux.contextKey, val <not Stringer>).WithValue(type string, val <not Stringer>).WithValue(type string, val <not Stringer>)\ndevlm-identity: 2024/10/19 23:28:55 main.go:66: Debug - Request: POST /api/v1/users/login\ndevlm-identity: 2024/10/19 23:28:55 main.go:67: Debug - Headers: map[Accept:[*/*] Content-Length:[51] Content-Type:[application/json] User-Agent:[curl/7.81.0]]\ndevlm-identity: 2024/10/19 23:28:55 main.go:70: Debug - Raw request body: {"username":"uniqueuser","password":"Password123!"}\ndevlm-identity: 2024/10/19 23:28:55 auth.go:135: Raw request body: {"username":"uniqueuser","password":"Password123!"}\n2024/10/19 23:28:55 Started POST /api/v1/users/login\n2024/10/19 23:28:55 Request Headers: map[Accept:[*/*] Content-Length:[51] Content-Type:[application/json] User-Agent:[curl/7.81.0]]\n2024/10/19 23:28:55 Request Body: {"username":"uniqueuser","password":"Password123!"}\ndevlm-identity: 2024/10/19 23:28:55 auth.go:149: Login attempt for username: uniqueuser or email: \ndevlm-identity: 2024/10/19 23:28:55 user.go:298: Getting user by username: uniqueuser\nPostgresDB: 2024/10/19 23:28:55 postgres.go:244: Retrieved user by username: ID=8b97c0d1-0159-4937-a223-2d2c61455e06, Username=uniqueuser\ndevlm-identity: 2024/10/19 23:28:55 user.go:304: Retrieved user by username: ID=8b97c0d1-0159-4937-a223-2d2c61455e06, Username=uniqueuser, Role=user\ndevlm-identity: 2024/10/19 23:28:55 auth.go:181: Verifying password for user: uniqueuser\ndevlm-identity: 2024/10/19 23:28:55 auth.go:202: Login successful for user: uniqueuser, Role: user\n2024/10/19 23:28:55 Completed POST /api/v1/users/login in 52.176401ms with status 200\n2024/10/19 23:28:55 Response Headers: map[Content-Type:[application/json]]\n2024/10/19 23:28:55 Response Body: {"refresh_token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MzAwMDY5MzUsInVzZXJfaWQiOiI4Yjk3YzBkMS0wMTU5LTQ5MzctYTIyMy0yZDJjNjE0NTVlMDYifQ.9qTIv4k6EuOb3u6Y4vcitfL8KpyYlPAImhd8ixOfpUQ","token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3Mjk0ODg1MzUsInJvbGUiOiJ1c2VyIiwidXNlcl9pZCI6IjhiOTdjMGQxLTAxNTktNDkzNy1hMjIzLTJkMmM2MTQ1NWUwNiIsInVzZXJuYW1lIjoidW5pcXVldXNlciJ9.a2xC0LJt4V-GK6Y9ZwyXPR59vCexxAPNuit7zvxd2wk"}\n2024/10/19 23:28:55 BodyParserMiddleware: Exiting for request: POST /api/v1/users/login\n']

Generating next step (Iteration 8)...
Response usage: Usage(input_tokens=13614, output_tokens=255)
LLM response:
<CoT>
Given that we have successfully registered a new user and logged in with their credentials, we have completed the main tasks requested by the user. The registration and login processes are working as expected. The next logical step would be to test a protected endpoint to ensure that the authentication system is fully functional. However, we don't have specific information about protected endpoints in this project.

Since we've accomplished the primary goals, it's appropriate to check with the user if there are any additional tasks or if they're satisfied with the current results. If they have no further requests, we can conclude the testing session.
</CoT>

ACTION: CHAT: We've successfully tested user registration and login. The system is working as expected. Are there any specific protected endpoints you'd like me to test using the authentication token we received, or is there anything else you'd like me to verify or test?

GOAL: Confirm with the user if there are any additional tasks or if the current testing is sufficient.

REASON: We've completed the main tasks of testing registration and login. It's important to check with the user for any additional requirements or to confirm if the current testing meets their needs before concluding the session.

Asking for help with the question: We've successfully tested user registration and login. The system is working as expected. Are there any specific protected endpoints you'd like me to test using the authentication token we received, or is there anything else you'd like me to verify or test?
Please provide your response to the model's question: We're done  
Response usage: Usage(input_tokens=12583, output_tokens=179)
History brief response: {
    "key_events": [
        "User requested to test the /registration endpoint using CURL.",
        "Initial attempt to register a user failed due to existing username.",
        "Second attempt to register a user failed due to existing email.",
        "Third attempt to register a user failed due to password complexity requirements (missing uppercase letter).",
        "Fourth attempt to register a user failed due to password complexity requirements (missing special character).",
        "User registration was successful with a password meeting all requirements.",
        "Login functionality was tested successfully with the newly created user.",
        "Authentication tokens (access token and refresh token) were received upon successful login.",
        "Testing of registration and login functionalities completed successfully.",
        "User confirmed that testing was complete and no further actions were required."
    ]
}
Updated history brief.
Running processes for debug: ['go run cmd/api/main.go']
Process outputs for debug: []

Generating next step (Iteration 9)...
Response usage: Usage(input_tokens=13973, output_tokens=146)
LLM response:
<CoT>
The user has indicated that we're done with the testing process. We've successfully tested the registration and login functionalities, and the user hasn't requested any further testing or verification. Given this information and following the directives, particularly directive 7 about not repeating actions unnecessarily, the most appropriate next step is to conclude the testing session.
</CoT>

ACTION: DONE
GOAL: Conclude the testing session as requested by the user.
REASON: The user has confirmed that we're done with the testing process. We've successfully completed the main tasks of testing registration and login, and the user hasn't requested any additional tests or verifications.

Test and debug mode completed.
Terminated process group: go run cmd/api/main.go
(myenv) orencollaco@Oren-Dell-Ubuntu:~/GitHub/devlm$ 
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