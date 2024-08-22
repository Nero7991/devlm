#!/bin/bash

set -euo pipefail

# Function to build Go services
build_go_services() {
    echo "Building Go services..."
    local services=("api_gateway" "action_executor" "code_execution_engine" "backend_service" "monitoring_service")
    for service in "${services[@]}"; do
        go build -o "bin/${service}" "cmd/${service}/main.go"
    done
}

# Function to build Python services
build_python_services() {
    echo "Building Python services..."
    python3 -m venv venv
    source venv/bin/activate
    pip install -r requirements.txt
    local services=("llm_service" "data_processor" "ml_service")
    for service in "${services[@]}"; do
        python -m py_compile "${service}/main.py"
    done
    deactivate
}

# Function to build Docker images
build_docker_images() {
    echo "Building Docker images..."
    local services=("api_gateway" "llm_service" "action_executor" "code_execution_engine" "backend_service" "data_processor" "monitoring_service" "ml_service")
    for service in "${services[@]}"; do
        docker build -t "devlm/${service}:latest" -f "docker/${service}.Dockerfile" .
        if [ $? -ne 0 ]; then
            log_error "Failed to build Docker image for ${service}"
            return 1
        fi
    done
}

# Function to check if required tools are installed
check_dependencies() {
    echo "Checking dependencies..."
    local deps=("go" "python3" "docker" "pip")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" >/dev/null 2>&1; then
            echo >&2 "$dep is not installed. Please install it and try again."
            exit 1
        fi
    done
    echo "All dependencies are installed."

    # Check versions of dependencies
    local go_version=$(go version | awk '{print $3}' | cut -c 3-)
    local python_version=$(python3 --version | awk '{print $2}')
    local docker_version=$(docker --version | awk '{print $3}' | cut -d',' -f1)

    if [[ "$(printf '%s\n' "1.16" "$go_version" | sort -V | head -n1)" != "1.16" ]]; then
        echo >&2 "Go version 1.16 or higher is required. Current version: $go_version"
        exit 1
    fi

    if [[ "$(printf '%s\n' "3.7" "$python_version" | sort -V | head -n1)" != "3.7" ]]; then
        echo >&2 "Python version 3.7 or higher is required. Current version: $python_version"
        exit 1
    fi

    if [[ "$(printf '%s\n' "20.10" "$docker_version" | sort -V | head -n1)" != "20.10" ]]; then
        echo >&2 "Docker version 20.10 or higher is required. Current version: $docker_version"
        exit 1
    fi
}

# Function to clean up build artifacts
cleanup() {
    echo "Cleaning up build artifacts..."
    rm -rf bin/*
    rm -rf venv
    find . -type d -name "__pycache__" -exec rm -rf {} +
    rm -f coverage.out coverage.xml build_report.txt
}

# Function to log errors
log_error() {
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

# Function to log info messages
log_info() {
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

# Function to run tests
run_tests() {
    log_info "Running tests..."
    go test -v -race -coverprofile=coverage.out ./...
    if [ $? -ne 0 ]; then
        log_error "Go tests failed"
        return 1
    fi
    source venv/bin/activate
    pytest --cov=. --cov-report=xml
    if [ $? -ne 0 ]; then
        log_error "Python tests failed"
        deactivate
        return 1
    fi
    deactivate
}

# Function to generate build report
generate_build_report() {
    log_info "Generating build report..."
    echo "Build Report" > build_report.txt
    echo "============" >> build_report.txt
    echo "Go version: $(go version)" >> build_report.txt
    echo "Python version: $(python3 --version)" >> build_report.txt
    echo "Docker version: $(docker --version)" >> build_report.txt
    echo "Build timestamp: $(date)" >> build_report.txt
    echo "Git commit: $(git rev-parse HEAD)" >> build_report.txt
    echo "Test coverage:" >> build_report.txt
    go tool cover -func=coverage.out >> build_report.txt
    echo "Python test coverage:" >> build_report.txt
    cat coverage.xml >> build_report.txt

    # Add individual service build details
    echo "Service Build Details:" >> build_report.txt
    local services=("api_gateway" "llm_service" "action_executor" "code_execution_engine" "backend_service" "data_processor" "monitoring_service" "ml_service")
    for service in "${services[@]}"; do
        echo "  ${service}:" >> build_report.txt
        echo "    Binary size: $(du -h bin/${service} 2>/dev/null | cut -f1)" >> build_report.txt
        echo "    Docker image size: $(docker image inspect devlm/${service}:latest --format='{{.Size}}' | numfmt --to=iec-i --suffix=B --format="%.2f")" >> build_report.txt
    done
}

# Function to run parallel build steps
run_parallel_build() {
    log_info "Running parallel build steps..."
    build_go_services &
    build_python_services &
    wait
}

# Main build function
main() {
    log_info "Starting build process..."

    cleanup
    check_dependencies

    run_parallel_build
    if [ $? -ne 0 ]; then
        log_error "Failed to build services"
        exit 1
    fi

    if ! run_tests; then
        log_error "Tests failed"
        exit 1
    fi

    if ! build_docker_images; then
        log_error "Failed to build Docker images"
        exit 1
    fi

    generate_build_report

    log_info "Build process completed successfully."
}

# Load custom configuration
if [ -f ".buildrc" ]; then
    source .buildrc
fi

# Parse command-line arguments
while getopts ":hcgtpd:" opt; do
    case ${opt} in
        h )
            echo "Usage: $0 [-h] [-c] [-g] [-t] [-p] [-d <service_name>]"
            echo "  -h  Display this help message"
            echo "  -c  Clean build artifacts only"
            echo "  -g  Build Go services only"
            echo "  -t  Run tests only"
            echo "  -p  Run parallel build"
            echo "  -d  Build Docker image for specific service"
            exit 0
            ;;
        c )
            cleanup
            exit 0
            ;;
        g )
            build_go_services
            exit 0
            ;;
        t )
            run_tests
            exit 0
            ;;
        p )
            run_parallel_build
            exit 0
            ;;
        d )
            docker build -t "devlm/${OPTARG}:latest" -f "docker/${OPTARG}.Dockerfile" .
            exit 0
            ;;
        \? )
            echo "Invalid option: $OPTARG" 1>&2
            exit 1
            ;;
        : )
            echo "Invalid option: $OPTARG requires an argument" 1>&2
            exit 1
            ;;
    esac
done
shift $((OPTIND -1))

# Run the main function if no specific options were provided
if [ $OPTIND -eq 1 ]; then
    main
fi