#!/bin/bash

set -e

# Function to run Go tests
run_go_tests() {
    echo "Running Go tests..."
    go test ./... -v -cover -race -timeout 15m -coverprofile=coverage.out -covermode=atomic
    return $?
}

# Function to run Python tests
run_python_tests() {
    echo "Running Python tests..."
    python3 -m pytest tests/ -v --cov=. --cov-report=xml --cov-report=term --durations=0 --timeout=900 --maxfail=5 --tb=short
    return $?
}

# Function to run integration tests
run_integration_tests() {
    echo "Running integration tests..."
    docker-compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from integration_tests
    local exit_code=$?
    docker-compose -f docker-compose.test.yml down --volumes --remove-orphans
    return $exit_code
}

# Function to run performance tests
run_performance_tests() {
    echo "Running performance tests..."
    locust -f performance_tests/locustfile.py --headless -u 300 -r 30 --run-time 15m --print-stats --only-summary --csv=locust_results --host=http://localhost:8080
    return $?
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install dependencies
install_dependencies() {
    if ! command_exists go; then
        echo "Installing Go..."
        wget https://golang.org/dl/go1.18.linux-amd64.tar.gz
        sudo tar -C /usr/local -xzf go1.18.linux-amd64.tar.gz
        export PATH=$PATH:/usr/local/go/bin
        rm go1.18.linux-amd64.tar.gz
    fi

    if ! command_exists python3; then
        echo "Installing Python3..."
        sudo apt-get update && sudo apt-get install -y python3 python3-pip python3-venv
    fi

    if ! command_exists docker-compose; then
        echo "Installing Docker Compose..."
        sudo curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
    fi

    python3 -m venv venv
    source venv/bin/activate
    pip install --upgrade pip
    pip install pytest pytest-cov pytest-timeout locust
}

# Main function to run all tests
run_all_tests() {
    install_dependencies

    local failed_tests=()

    run_go_tests || failed_tests+=("Go tests")
    run_python_tests || failed_tests+=("Python tests")
    run_integration_tests || failed_tests+=("Integration tests")
    run_performance_tests || failed_tests+=("Performance tests")

    if [ ${#failed_tests[@]} -ne 0 ]; then
        echo "The following tests failed:"
        for test in "${failed_tests[@]}"; do
            echo "- $test"
        done
        exit 1
    fi

    echo "All tests passed successfully!"
}

# Function to generate test report
generate_test_report() {
    echo "Generating test report..."
    {
        echo "Test Report"
        echo "============"
        echo ""
        
        echo "Go Test Coverage:"
        go tool cover -func=coverage.out
        echo ""
        
        echo "Python Test Coverage:"
        coverage report -m
        echo ""
        
        echo "Performance Test Summary:"
        tail -n 10 locust_results_stats.csv
        
        echo "Integration Test Results:"
        cat integration_test_results.log
        
    } > test_report.txt
    
    echo "Test report generated: test_report.txt"
}

# Function to run specific test suite
run_specific_test() {
    install_dependencies
    case "$1" in
        go)
            run_go_tests
            ;;
        python)
            run_python_tests
            ;;
        integration)
            run_integration_tests
            ;;
        performance)
            run_performance_tests
            ;;
        *)
            echo "Invalid test type. Usage: $0 [go|python|integration|performance|report|parallel]"
            exit 1
            ;;
    esac
}

# Function to clean up test artifacts
cleanup_test_artifacts() {
    echo "Cleaning up test artifacts..."
    rm -f coverage.out
    rm -f .coverage
    rm -f locust_results_*.csv
    rm -f test_report.txt
    rm -f integration_test_results.log
}

# Function to run tests in parallel
run_parallel_tests() {
    install_dependencies

    local pids=()
    local failed_tests=()

    run_go_tests & pids+=($!)
    run_python_tests & pids+=($!)
    run_integration_tests & pids+=($!)
    run_performance_tests & pids+=($!)

    for pid in "${pids[@]}"; do
        wait $pid || failed_tests+=("Test process $pid")
    done

    if [ ${#failed_tests[@]} -ne 0 ]; then
        echo "The following test processes failed:"
        for test in "${failed_tests[@]}"; do
            echo "- $test"
        done
        exit 1
    fi

    echo "All tests passed successfully!"
}

# Main execution
if [ $# -eq 1 ]; then
    if [ "$1" = "report" ]; then
        generate_test_report
    elif [ "$1" = "parallel" ]; then
        run_parallel_tests
        generate_test_report
    else
        run_specific_test "$1"
    fi
elif [ $# -eq 0 ]; then
    run_all_tests
    generate_test_report
else
    echo "Usage: $0 [go|python|integration|performance|report|parallel]"
    exit 1
fi

# Clean up test artifacts
cleanup_test_artifacts

# Deactivate virtual environment if it was activated
if [ -n "$VIRTUAL_ENV" ]; then
    deactivate
fi