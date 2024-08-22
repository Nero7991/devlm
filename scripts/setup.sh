#!/bin/bash

set -e

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

install_docker() {
    echo "Installing Docker..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update
        sudo apt-get install -y apt-transport-https ca-certificates curl software-properties-common
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
        sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
        sudo apt-get update
        sudo apt-get install -y docker-ce docker-ce-cli containerd.io
        sudo systemctl start docker
        sudo systemctl enable docker
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install --cask docker
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please install Docker Desktop for Windows manually from https://www.docker.com/products/docker-desktop"
        exit 1
    else
        echo "Unsupported OS for automatic Docker installation. Please install Docker manually."
        exit 1
    fi

    if ! docker --version; then
        echo "Docker installation failed. Please check the error messages and try again."
        exit 1
    fi
}

install_golang() {
    echo "Installing Golang..."
    local go_version=${GO_VERSION:-"1.17.5"}
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        wget "https://golang.org/dl/go${go_version}.linux-amd64.tar.gz"
        sudo tar -C /usr/local -xzf "go${go_version}.linux-amd64.tar.gz"
        rm "go${go_version}.linux-amd64.tar.gz"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install go
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please install Go manually from https://golang.org/dl/"
        exit 1
    else
        echo "Unsupported OS for automatic Golang installation. Please install Golang manually."
        exit 1
    fi
    
    if ! go version; then
        echo "Golang installation failed. Please check the error messages and try again."
        exit 1
    fi

    if [ -z "$GOPATH" ]; then
        echo 'export GOPATH=$HOME/go' >> ~/.bashrc
        echo 'export PATH=$PATH:$GOPATH/bin:/usr/local/go/bin' >> ~/.bashrc
        source ~/.bashrc
    fi
}

install_python() {
    echo "Installing Python..."
    local python_version=${PYTHON_VERSION:-"3.9"}
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update
        sudo apt-get install -y python${python_version} python3-pip python3-venv
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install python@${python_version}
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please install Python manually from https://www.python.org/downloads/"
        exit 1
    else
        echo "Unsupported OS for automatic Python installation. Please install Python manually."
        exit 1
    fi
    
    if ! python3 --version; then
        echo "Python installation failed. Please check the error messages and try again."
        exit 1
    fi

    pip3 install --upgrade pip
    pip3 install virtualenv
}

install_redis() {
    echo "Installing Redis..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update
        sudo apt-get install -y redis-server
        sudo systemctl start redis-server
        sudo systemctl enable redis-server
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install redis
        brew services start redis
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please install Redis manually from https://redis.io/download"
        exit 1
    else
        echo "Unsupported OS for automatic Redis installation. Please install Redis manually."
        exit 1
    fi
    
    if ! redis-cli --version; then
        echo "Redis installation failed. Please check the error messages and try again."
        exit 1
    fi
}

install_postgresql() {
    echo "Installing PostgreSQL..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update
        sudo apt-get install -y postgresql postgresql-contrib
        sudo systemctl start postgresql
        sudo systemctl enable postgresql
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew install postgresql
        brew services start postgresql
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please install PostgreSQL manually from https://www.postgresql.org/download/windows/"
        exit 1
    else
        echo "Unsupported OS for automatic PostgreSQL installation. Please install PostgreSQL manually."
        exit 1
    fi
    
    if ! psql --version; then
        echo "PostgreSQL installation failed. Please check the error messages and try again."
        exit 1
    fi

    local db_name=${DB_NAME:-"devlm"}
    local db_user=${DB_USER:-"devlm_user"}
    local db_password=${DB_PASSWORD:-"devlm_password"}

    sudo -u postgres psql -c "CREATE DATABASE $db_name;"
    sudo -u postgres psql -c "CREATE USER $db_user WITH PASSWORD '$db_password';"
    sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE $db_name TO $db_user;"
}

log_error() {
    echo "[ERROR] $1" >&2
}

setup_project_structure() {
    echo "Setting up project structure..."
    mkdir -p api_gateway llm_service action_executor code_execution_engine
    touch api_gateway/main.go llm_service/main.py action_executor/main.go code_execution_engine/main.go
}

install_project_dependencies() {
    echo "Installing project dependencies..."
    go mod init github.com/Nero7991/devlm
    go get -u github.com/gin-gonic/gin
    go get -u github.com/go-redis/redis/v8
    go get -u github.com/lib/pq

    python3 -m venv venv
    source venv/bin/activate
    pip install fastapi uvicorn redis asyncpg
    pip freeze > requirements.txt
    deactivate
}

update_system_packages() {
    echo "Updating system packages..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        sudo apt-get update
        sudo apt-get upgrade -y
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        brew update
        brew upgrade
    elif [[ "$OSTYPE" == "msys"* ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "Please update your system manually."
    else
        echo "Unsupported OS for automatic system update. Please update your system manually."
    fi
}

configure_environment() {
    echo "Configuring environment variables..."
    cat << EOF > .env
POSTGRES_DB=${DB_NAME:-devlm}
POSTGRES_USER=${DB_USER:-devlm_user}
POSTGRES_PASSWORD=${DB_PASSWORD:-devlm_password}
REDIS_HOST=${REDIS_HOST:-localhost}
REDIS_PORT=${REDIS_PORT:-6379}
API_GATEWAY_PORT=${API_GATEWAY_PORT:-8080}
LLM_SERVICE_PORT=${LLM_SERVICE_PORT:-8000}
ACTION_EXECUTOR_PORT=${ACTION_EXECUTOR_PORT:-8001}
CODE_EXECUTION_ENGINE_PORT=${CODE_EXECUTION_ENGINE_PORT:-8002}
LLM_API_KEY=${LLM_API_KEY:-your_llm_api_key_here}
EOF
    echo "Environment variables configured. Please review and update the .env file as needed."
}

check_dependencies() {
    local dependencies=("docker" "go" "python3" "redis-cli" "psql")
    local missing_deps=()

    for dep in "${dependencies[@]}"; do
        if ! command_exists "$dep"; then
            missing_deps+=("$dep")
        fi
    done

    if [ ${#missing_deps[@]} -ne 0 ]; then
        echo "The following dependencies are missing:"
        for dep in "${missing_deps[@]}"; do
            echo "- $dep"
        done
        echo "Please install them before proceeding."
        exit 1
    fi
}

main() {
    echo "Starting DevLM setup..."

    update_system_packages

    if ! command_exists docker; then
        install_docker
    else
        echo "Docker is already installed."
    fi

    if ! command_exists go; then
        install_golang
    else
        echo "Golang is already installed."
    fi

    if ! command_exists python3; then
        install_python
    else
        echo "Python is already installed."
    fi

    if ! command_exists redis-cli; then
        install_redis
    else
        echo "Redis is already installed."
    fi

    if ! command_exists psql; then
        install_postgresql
    else
        echo "PostgreSQL is already installed."
    fi

    check_dependencies

    setup_project_structure
    install_project_dependencies
    configure_environment

    echo "DevLM setup completed successfully!"
}

main || log_error "Setup failed. Please check the error messages above."