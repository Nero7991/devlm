# DevLM - Software Development with LLMs
### Oren's crude attempt at somewhat fully automating software development, testing and integration in his personal projects

Demo video of DevLM modifying and testing the /api/v1/users/register endpoint to /new in [devlm-identity service](https://github.com/Nero7991/devlm-identity) which was fully developed by Claude using DevLM: https://www.youtube.com/watch?v=_xGed1biVoM

## Major Issues

- The needle in the haystack problem causes loops and repeated actions when a task has many steps. (Only tested with Claude Sonnet 3.5)
- Claude Sonnet 3.5 is unnecessarly verbose. Need a more concise model.

## Vision and background

I'm one of the laziest person I know when it comes to completing personal projects that don't have a deadline or some overseer. I try to document my projects but get bored of documenting and they just sit in the dust once I've moved on to other ones which are destined again to the same fate. My gaol with DevLM was to have it complete(code development, integration, testing and documentation) the projects I've never completed. I hoped that a reasoning model with technical knowlegde related to my personal projects would solve this. Introduce, DevLM, it uses LLMs (Claude Sonnet 3.5, need to test with other local models) and enables them to take actions. It provides dynamic project context so that LLMs can actually take the proper actions to complete a request. I've thrown in features to realise my vision for DevLM and it does do a pretty good job (it blew me away intially) but still fails at the vision. (Request it to add a feature or fix a bug and it does but perfectly and most times I need to intervene to get it to do it the way I want it to)

The problem is that the LLM I'm using (Claude Sonnet 3.5) is not able to completely recall key events from the action history (context) (needle in the haystack). For example, when it runs an API process, it also receives the output for the process. When a test request is made using CURL, the server process output is provided to it. For outputs, in the recents actions (last 2), it is able to recall the output and take the right actions. But I'm giving it 15 recent actions as context (read below for details on context) and it is not able to recall the output for the older actions. This leads to it repeating certain actions and falling into a loop if the there are many steps to a particular  task/request. This is using just 50k context and claude supports 200k. What's the use of 200k context if the LLM can't recall details in just ~50k?

I realise that models will improve over time (maybe I should try o1 preview or Opus 3.5 when its out) and once this context recall issue is solved, DevLM might be a very powerful tool to me. Edit: Looking at the differential transformer paper from Microsoft, future models might be able to handle this better.

## Improvements

Model improvements:
- Pretrain a model to learn the action and directives and define actions as tokens so that they don't have to be prompted and the LLM is more familiar with the actions. 
- A model that uses the newly introduced differential transformer architecture for better attention on key events in the context for next action/decision. (better needle in the haystack performance)
- Implement support for llama.cpp to use local models

## Actions LLM can take

The DevLM bootstrap script allows the LLM to take the following actions, for every action the LLM provides a reason, goal and chain of thought, each action is decided by the LLM based on the context provided (look below for what context is provided):

List of actions: (to look at prompt, see [bootstrap.py lines 2234-2253](https://github.com/Nero7991/devlm/blob/main/bootstrap.py#L2234-L2253), code: [bootstrap.py lines 2314 - 2778](https://github.com/Nero7991/devlm/blob/main/bootstrap.py#L2314-L2778))

List of actions: (to look at code, see bootstrap.py lines 2234 - 2253, 2314 - 2778)

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

6. **MULTI**: Read multiple files and modify one of them [LLM is prompted to output the entire file with the changes, tried using lines diffs but it was too unreliable].
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

## Context for Decision Making

The LLM receives the following context to decide its next action (To look at code, see bootstrap.py lines 2192 - 2266):

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

3. Add API key for Anthropic/Project ID and region for Google Cloud in the `devlm.env` file depending on the source you want to use. You can also specify the API key/Project ID and region via command line arguments (override the values in the `devlm.env` file). To get API key for Anthropic models, see [Anthropic API](https://www.anthropic.com/api). If you want to use Google Cloud, you can get the project ID and region from the [Google Cloud Console](https://console.cloud.google.com/), install gcloud by following the instructions [here](https://cloud.google.com/sdk/docs/install) and run `gcloud auth application-default login` to login to your Google Cloud account. You also need to enable the Vertex AI API and Claude Sonnet 3.5 API in the [Google Cloud Vertex API](https://cloud.google.com/vertex-ai/generative-ai/docs/partner-models/use-claude).

4. Run the script:
```
python3 bootstrap.py --mode [generate|test] --source [gcloud|anthropic] --project-path /path/to/your/project [--frontend] [--model MODEL] [--api-key API_KEY] [--project-id PROJECT_ID] [--region REGION]

Arguments:
1. --mode: Required. Choose between "generate" or "test".
2. --source: Required. Choose between "gcloud" or "anthropic".
3. --project-path: Optional. Specifies the path to the project directory (default is current directory).
4. --frontend: Optional flag. Enable frontend testing if included.
5. --model: Optional. Specifies the model to use (default is "claude").
6. --api-key: Optional. Specifies the API key for Anthropic, by default it will use the API key in the `devlm.env` file (only used if source is 'anthropic').
7. --project-id: Optional. Specifies the Google Cloud project ID, by default it will use the project ID in the `devlm.env` file (only used if source is 'gcloud').
8. --region: Optional. Specifies the Google Cloud region, by default it will use the region in the `devlm.env` file (only used if source is 'gcloud').

```

There are two modes:
- `generate`: The LLM will generate a project directory structure based on the project summary and create empty files. Originally, plan was to generate the initial code in generate mode as well but it was too unreliable. Test mode does that much better.
- `test`: Develop, test and debug mode. Chat with the LLM to develop your project.

### Example of develop, test and debug mode (--mode test) in action:

The complete code (except initial project directory structure which was generated using generate mode) in the devlm-identity was developed, tested and debugged using this mode. Available at [devlm-identity](https://github.com/nero7991/devlm-identity).

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

### Example of generate mode (--mode generate) in action:
```
(myenv) orencollaco@Oren-Dell-Ubuntu:~/GitHub/devlm$ python3 bootstrap.py --mode generate --source gcloud --project-id "devlm-434202" --region "us-east5" --project-path ~/GitHub/devlm/devlm-ml/
Using Claude via Google Cloud. Project ID: devlm-434202, Region: us-east5
Working directory set to: /home/orencollaco/GitHub/devlm/devlm-ml/
Created .devlm/chat.txt. You can write notes in this file to communicate with the LLM.
Project directory structure has been generated.

Do you want to generate/update the project directory structure? (yes/no) [default: no]: yes
Response usage: Usage(input_tokens=1085, output_tokens=754)
Suggested project structure:
{
  "": [
    "README.md",
    "go.mod",
    "go.sum",
    "Makefile",
    "Dockerfile",
    ".gitignore",
    ".env.example"
  ],
  "cmd": {
    "api": [
      "main.go"
    ]
  },
  "internal": {
    "api": [
      "router.go",
      "handlers.go"
    ],
    "codegeneration": [
      "generator.go"
    ],
    "llm": [
      "client.go",
      "anthropic.go",
      "vertexai.go"
    ],
    "projectrefinement": [
      "refiner.go"
    ],
    "config": [
      "config.go"
    ]
  },
  "pkg": {
    "models": [
      "code_generation.go",
      "project.go"
    ],
    "middleware": [
      "ml_middleware.go"
    ],
    "database": [
      "postgres.go"
    ]
  },
  "api": [
    "openapi.yaml"
  ],
  "config": [
    "config.yaml"
  ],
  "deployments": {
    "kubernetes": [
      "service.yaml",
      "deployment.yaml"
    ]
  },
  "scripts": [
    "setup.sh",
    "test.sh"
  ],
  "docs": [
    "architecture.md",
    "api.md"
  ],
  "test": {
    "integration": [
      "integration_test.go"
    ],
    "unit": [
      "unit_test.go"
    ]
  }
}

Explanation of changes and additions:

1. Root directory:
   - Added essential files like README.md, go.mod, go.sum, Makefile, Dockerfile, .gitignore, and .env.example.

2. cmd/api:
   - Kept main.go as the entry point for the service.

3. internal directory:
   - Created an api package with router.go and handlers.go to separate routing and handler logic.
   - Renamed llm_client.go to client.go and added separate files for Anthropic and Vertex AI implementations.
   - Added a config package with config.go for centralized configuration management.

4. pkg directory:
   - Kept the existing structure for models, middleware, and database.

5. api directory:
   - Moved openapi.yaml here for better organization of API-related files.

6. config directory:
   - Kept config.yaml for service configuration.

7. deployments directory:
   - Created a kubernetes subdirectory for Kubernetes deployment files.

8. scripts directory:
   - Added setup.sh and test.sh for common development and testing tasks.

9. docs directory:
   - Added architecture.md and api.md for detailed documentation.

10. test directory:
    - Created separate directories for integration and unit tests.

This structure improves upon the original by:
- Following Go project layout conventions more closely.
- Separating concerns more effectively (e.g., API handlers, router, and LLM client implementations).
- Adding necessary configuration and documentation files.
- Including directories for tests, scripts, and deployment configurations.
- Providing a clearer separation between internal and public (pkg) packages.

This structure should enhance maintainability, scalability, and adherence to best practices for Go projects.

Do you want to apply the suggested structure? Warning: This will overwrite the current structure deleting all files and directories in the project directory (yes/no) [default: no]: yes
Technical brief saved to .devlm/project_technical_brief.json

Do you want to generate code/content for the files? [NOT RECOMMENDED as it is unreliable, use test mode instead to develop code/content for the project] (yes/no) [default: no]: 
Skipping code/content generation. Restarts DevLM in test mode using --mode test.
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

- `--frontend`: Enable UI testing actions
- `--api-key`: Anthropic API key (if using anthropic source)
- `--project-id`: Google Cloud project ID (if using gcloud source)
- `--region`: Google Cloud region (if using gcloud source)

## Known Limitations (this will improve as model improves and needle in a haystack retrival gets better)

- May produce inconsistent or incorrect code
- Limited understanding of complex project structures
- Potential for generating non-functional or syntactically incorrect code
- May not adhere to best practices or specific coding standards
- Frontend testing features are experimental and may not work as expected

## Contributing

As this project is in its early stages, we're primary focus on getting the vision and core functionality right. Feedback and issue reports are most welcome.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.