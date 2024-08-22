# Contributing to DevLM

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/devlm.git`
3. Create a new branch: `git checkout -b feature/your-feature-name`
4. Sync your fork with the upstream repository:
   ```
   git remote add upstream https://github.com/Nero7991/devlm.git
   git fetch upstream
   git merge upstream/main
   ```

## Setting up the Development Environment

### Prerequisites

- Go 1.20 or later
- Python 3.9 or later
- Docker
- Redis
- PostgreSQL

### Setup

1. Install Go dependencies:
   ```
   go mod tidy
   ```

2. Install Python dependencies:
   ```
   pip install -r requirements.txt
   ```

3. Set up environment variables:
   ```
   cp .env.example .env
   ```
   Edit the `.env` file with your local configuration.

4. Start Redis and PostgreSQL:
   ```
   docker-compose up -d redis postgres
   ```

5. Initialize the database:
   ```
   go run cmd/migrate/main.go
   ```

### Automated Setup Script

```bash
#!/bin/bash

# Automated setup script for DevLM

# Check for required tools
command -v go >/dev/null 2>&1 || { echo "Go is not installed. Aborting."; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "Python 3 is not installed. Aborting."; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Docker is not installed. Aborting."; exit 1; }

# Install Go dependencies
echo "Installing Go dependencies..."
go mod tidy

# Install Python dependencies
echo "Installing Python dependencies..."
pip install -r requirements.txt

# Set up environment variables
echo "Setting up environment variables..."
cp .env.example .env
echo "Please edit .env file with your local configuration."

# Start Redis and PostgreSQL
echo "Starting Redis and PostgreSQL..."
docker-compose up -d redis postgres

# Initialize the database
echo "Initializing the database..."
go run cmd/migrate/main.go

echo "Setup complete. Please review the .env file and make any necessary changes."
```

Save this script as `setup.sh` in the project root and make it executable with `chmod +x setup.sh`. Run it with `./setup.sh`.

### Troubleshooting

If you encounter issues during setup:

1. Ensure all prerequisites are installed and up-to-date
2. Check that environment variables are correctly set
3. Verify that Redis and PostgreSQL are running
4. Clear Go module cache: `go clean -modcache`
5. Rebuild Python virtual environment

For persistent issues, please open a GitHub issue with detailed information about your environment and the error messages.

## Development Workflow

1. Make your changes in your feature branch
2. Write tests for your changes
3. Run tests:
   ```
   go test ./...
   python -m pytest tests/
   ```
4. Ensure your code follows our [coding standards](./coding_standards.md)
5. Run linters:
   ```
   golangci-lint run
   flake8 .
   ```
6. Commit your changes:
   ```
   git commit -am "Add your commit message"
   ```
7. Push to your fork:
   ```
   git push origin feature/your-feature-name
   ```
8. Create a pull request from your fork to the main repository

## Pull Request Process

1. Ensure your code passes all tests and linting checks
2. Update the README.md with details of changes, if applicable
3. Increase the version numbers in any examples files and the README.md to the new version that this Pull Request would represent
4. Your pull request will be reviewed by maintainers, who may request changes or provide feedback
5. Once approved, your pull request will be merged

### Pull Request Checklist

```markdown
## Pull Request Checklist

Before submitting your pull request, please review and complete this checklist:

- [ ] I have read and followed the [CONTRIBUTING.md](CONTRIBUTING.md) guidelines
- [ ] My code follows the project's coding standards
- [ ] I have added/updated necessary documentation
- [ ] I have added/updated appropriate tests
- [ ] All new and existing tests pass
- [ ] I have checked for and resolved any merge conflicts
- [ ] I have updated the CHANGELOG.md file (if applicable)
- [ ] I have updated any relevant configuration files or environment variables
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] I have checked my code and corrected any misspellings

### Type of change

Please delete options that are not relevant.

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] This change requires a documentation update

### Description

Please provide a brief description of the changes in this pull request.

### Related Issue

If applicable, please link to the issue this pull request addresses.

### Additional Notes

Add any other context or screenshots about the pull request here.
```

### Handling Merge Conflicts

If your pull request has merge conflicts:

1. Fetch the latest changes from the upstream repository
2. Rebase your branch on the latest main:
   ```
   git fetch upstream
   git rebase upstream/main
   ```
3. Resolve conflicts in your local files
4. Stage the resolved files: `git add .`
5. Continue the rebase: `git rebase --continue`
6. Force-push your changes: `git push -f origin feature/your-feature-name`

## Reporting Bugs

1. Check the issue tracker to see if the bug has already been reported
2. If not, create a new issue with a clear title and description
3. Include steps to reproduce the bug and any relevant error messages or logs
4. Provide information about your environment (OS, Go version, Python version, etc.)
5. Use the bug report template provided in the `.github/ISSUE_TEMPLATE/bug_report.md` file

## Suggesting Enhancements

1. Check the issue tracker to see if the enhancement has already been suggested
2. If not, create a new issue with a clear title and detailed description of the proposed enhancement
3. Explain why this enhancement would be useful to most DevLM users
4. Use the feature request template provided in the `.github/ISSUE_TEMPLATE/feature_request.md` file

## Code of Conduct

Please note that this project is released with a [Contributor Code of Conduct](./CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

## Documentation

1. Update documentation for any code changes you make
2. Follow the existing documentation style and format
3. If adding new features, include appropriate examples and explanations
4. Use clear, concise language and avoid jargon
5. Include inline comments for complex code sections
6. Update API documentation when changing interfaces

### Documentation Style Guide

- Use Markdown for all documentation files
- Keep line length to a maximum of 80 characters for better readability
- Use atx-style headers (# Header 1, ## Header 2, etc.)
- Use code blocks for code snippets, command-line examples, and configuration samples
- Provide links to external resources when appropriate

## Continuous Integration

1. All pull requests are automatically tested using GitHub Actions
2. Ensure your changes pass all CI checks before requesting a review
3. To debug CI failures locally:
   - Review the CI logs for specific error messages
   - Replicate the CI environment using Docker
   - Run the same commands locally as specified in the CI configuration

## Security

1. Follow the [security guidelines](./security.md) when developing
2. Report any security vulnerabilities according to our security policy
3. Participate in regular security audits:
   - Review dependencies for known vulnerabilities
   - Use static analysis tools to identify potential security issues
   - Conduct manual code reviews focused on security best practices

## Questions?

If you have any questions or need further clarification, please open an issue or contact the maintainers directly.

Thank you for contributing to DevLM!

## Additional Guidelines

### Branching Strategy

- Use feature branches for all new features and bug fixes
- Branch naming convention: `feature/short-description` or `bugfix/issue-number`
- Keep branches focused and short-lived

### Code Review Process

- All code changes require at least one approval from a maintainer
- Address review comments promptly
- Reviewers should provide constructive feedback and suggestions for improvement

### Release Process

1. Update CHANGELOG.md with a summary of changes
2. Create a new release branch: `release/vX.Y.Z`
3. Update version numbers in relevant files
4. Create a pull request for the release
5. After approval, merge the release branch into main
6. Tag the release: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
7. Push the tag: `git push origin vX.Y.Z`

### Versioning

We follow [Semantic Versioning](https://semver.org/). Version numbers should be in the format `MAJOR.MINOR.PATCH`.

### Performance Considerations

- Write efficient, optimized code
- Use benchmarks to measure performance impacts of changes
- Consider scalability when designing new features

### Accessibility

- Ensure all new features are accessible
- Follow WCAG 2.1 guidelines for web interfaces
- Test with screen readers and other assistive technologies

### Internationalization and Localization

- Use string literals that can be easily translated
- Avoid hardcoding text in the UI
- Consider right-to-left languages in layouts

### Data Privacy

- Follow data protection regulations (e.g., GDPR, CCPA)
- Minimize collection of personal data
- Implement data retention and deletion policies

### Third-party Integrations

- Document all third-party integrations
- Ensure proper licensing for all dependencies
- Regularly update and audit third-party libraries

### Community Engagement

- Respond to issues and pull requests in a timely manner
- Encourage and mentor new contributors
- Recognize valuable contributions from community members

By following these guidelines, we can maintain a high-quality, collaborative project that benefits all users and contributors of DevLM.