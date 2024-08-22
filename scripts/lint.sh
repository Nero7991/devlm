#!/bin/bash

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to lint Go code
lint_go_code() {
    echo "Linting Go code..."
    if ! command_exists golangci-lint; then
        echo "golangci-lint not found. Installing..."
        GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    fi

    if [[ -f ".golangci.yml" ]]; then
        golangci-lint run --config=.golangci.yml ./... --timeout=5m
    else
        golangci-lint run ./... --timeout=5m
    fi
}

# Function to lint Python code
lint_python_code() {
    echo "Linting Python code..."
    local pip_command="pip"
    if command_exists pip3; then
        pip_command="pip3"
    fi

    local tools=("pylint" "flake8" "black" "mypy")
    for tool in "${tools[@]}"; do
        if ! command_exists "$tool"; then
            echo "$tool not found. Installing..."
            $pip_command install "$tool"
        fi
    done

    if [[ -n "$VIRTUAL_ENV" ]]; then
        echo "Using virtual environment: $VIRTUAL_ENV"
    else
        echo "Warning: Not running in a virtual environment. Consider using one for isolated dependencies."
    fi

    if [[ -f ".pylintrc" ]]; then
        pylint --rcfile=.pylintrc --recursive=y .
    else
        pylint --recursive=y .
    fi

    if [[ -f ".flake8" ]]; then
        flake8 --config=.flake8 .
    else
        flake8 .
    fi

    if [[ -f "pyproject.toml" ]]; then
        black --config pyproject.toml --check .
    else
        black --check .
    fi

    if [[ -f "mypy.ini" ]]; then
        mypy --config-file mypy.ini .
    else
        mypy .
    fi
}

# Function to lint shell scripts
lint_shell_scripts() {
    echo "Linting shell scripts..."
    if ! command_exists shellcheck; then
        echo "shellcheck not found. Installing..."
        if command_exists apt-get; then
            sudo apt-get update && sudo apt-get install -y shellcheck
        elif command_exists brew; then
            brew install shellcheck
        else
            echo "Unable to install shellcheck. Please install it manually."
            return 1
        fi
    fi

    if [[ $# -gt 0 ]]; then
        shellcheck -s bash -o all "$@"
    else
        shellcheck -s bash -o all ./*.sh
    fi
}

# Function to lint Dockerfile
lint_dockerfile() {
    echo "Linting Dockerfile..."
    if ! command_exists hadolint; then
        echo "hadolint not found. Using Docker to run hadolint..."
        if ! command_exists docker; then
            echo "Docker not found. Unable to run hadolint. Please install Docker or hadolint manually."
            return 1
        fi
        docker pull hadolint/hadolint
    fi

    for dockerfile in Dockerfile Dockerfile.*; do
        if [[ -f "$dockerfile" ]]; then
            echo "Linting $dockerfile"
            if command_exists hadolint; then
                hadolint --format json "$dockerfile" | jq .
            else
                docker run --rm -i hadolint/hadolint < "$dockerfile" | jq .
            fi
        fi
    done
}

# Function to run all linters
run_all_linters() {
    local exit_code=0

    lint_go_code || exit_code=$?
    lint_python_code || exit_code=$?
    lint_shell_scripts || exit_code=$?
    lint_dockerfile || exit_code=$?

    return $exit_code
}

# Function to run linters in parallel
run_parallel_linters() {
    local exit_code=0
    local pids=()

    lint_go_code & pids+=($!)
    lint_python_code & pids+=($!)
    lint_shell_scripts & pids+=($!)
    lint_dockerfile & pids+=($!)

    for pid in "${pids[@]}"; do
        wait $pid || exit_code=$?
    done

    return $exit_code
}

# Function to load custom configurations
load_custom_config() {
    local config_files=(".lintrc" ".lintrc.json" ".lintrc.yaml" ".lintrc.yml")
    for config_file in "${config_files[@]}"; do
        if [[ -f "$config_file" ]]; then
            case "$config_file" in
                *.json)
                    # Parse JSON config file
                    eval "$(jq -r 'to_entries | .[] | "export " + .key + "=\"" + (.value|tostring) + "\""' "$config_file")"
                    ;;
                *.yaml|*.yml)
                    # Parse YAML config file
                    eval "$(yq e 'to_entries | .[] | "export " + .key + "=\"" + (.value|tostring) + "\""' "$config_file")"
                    ;;
                *)
                    # Assume it's a shell script
                    # shellcheck disable=SC1090
                    source "$config_file"
                    ;;
            esac
            echo "Loaded custom configuration from $config_file"
            break
        fi
    done
}

# Main function
main() {
    local exit_code=0
    local parallel=false
    local log_file="lint_results.log"

    load_custom_config

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --parallel)
                parallel=true
                shift
                ;;
            --log)
                log_file="$2"
                shift 2
                ;;
            --help)
                echo "Usage: $0 [--parallel] [--log <file>] [--help] [go|python|shell|dockerfile]"
                echo "Options:"
                echo "  --parallel           Run linters in parallel"
                echo "  --log <file>         Specify a log file (default: lint_results.log)"
                echo "  --help               Show this help message"
                echo "  go                   Run Go linter"
                echo "  python               Run Python linters"
                echo "  shell                Run shell script linter"
                echo "  dockerfile           Run Dockerfile linter"
                exit 0
                ;;
            go|python|shell|dockerfile)
                break
                ;;
            *)
                echo "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done

    # Redirect output to both console and log file
    exec > >(tee -a "$log_file") 2>&1

    echo "Linting started at $(date)"

    if [[ $# -eq 0 ]]; then
        if $parallel; then
            run_parallel_linters
        else
            run_all_linters
        fi
        exit_code=$?
    else
        case "$1" in
            go)
                lint_go_code
                exit_code=$?
                ;;
            python)
                lint_python_code
                exit_code=$?
                ;;
            shell)
                shift
                lint_shell_scripts "$@"
                exit_code=$?
                ;;
            dockerfile)
                lint_dockerfile
                exit_code=$?
                ;;
            *)
                echo "Usage: $0 [--parallel] [--log <file>] [--help] [go|python|shell|dockerfile]"
                exit 1
                ;;
        esac
    fi

    echo "Linting process completed at $(date)"
    echo "Lint results have been saved to $log_file"

    return $exit_code
}

# Run the main function with any command-line arguments
main "$@"