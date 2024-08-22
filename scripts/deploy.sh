#!/bin/bash

set -e

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to deploy API Gateway
deploy_api_gateway() {
    echo "Deploying API Gateway..."
    docker run -d --name api_gateway -p "${API_GATEWAY_PORT:-8080}:8080" --network devlm-network \
        --health-cmd "curl -f http://localhost:8080/health || exit 1" \
        --health-interval 30s \
        --health-retries 3 \
        --health-timeout 10s \
        --restart unless-stopped \
        -e LOG_LEVEL=${API_GATEWAY_LOG_LEVEL:-info} \
        api_gateway:${API_GATEWAY_VERSION:-latest}
    check_health api_gateway "http://localhost:${API_GATEWAY_PORT:-8080}/health" 60
}

# Function to deploy LLM Service
deploy_llm_service() {
    echo "Deploying LLM Service..."
    docker run -d --name llm_service -p "${LLM_SERVICE_PORT:-8081}:8081" --network devlm-network \
        -e LLM_API_KEY=${LLM_API_KEY} \
        -e LLM_MODEL=${LLM_MODEL} \
        -e FALLBACK_MODEL=${FALLBACK_MODEL} \
        -e MAX_TOKENS=${MAX_TOKENS:-4096} \
        -e TEMPERATURE=${TEMPERATURE:-0.7} \
        --health-cmd "curl -f http://localhost:8081/health || exit 1" \
        --health-interval 30s \
        --health-retries 3 \
        --health-timeout 10s \
        --restart unless-stopped \
        llm_service:${LLM_SERVICE_VERSION:-latest}
    check_health llm_service "http://localhost:${LLM_SERVICE_PORT:-8081}/health" 60
}

# Function to deploy Action Executor
deploy_action_executor() {
    echo "Deploying Action Executor..."
    docker run -d --name action_executor --network devlm-network \
        -v ${PROJECT_PATH}:/app/project:ro \
        --cpus ${ACTION_EXECUTOR_CPUS:-1} --memory ${ACTION_EXECUTOR_MEMORY:-2g} --pids-limit ${ACTION_EXECUTOR_PIDS:-50} \
        --cap-drop ALL --security-opt no-new-privileges \
        --health-cmd "curl -f http://localhost:8082/health || exit 1" \
        --health-interval 30s \
        --health-retries 3 \
        --health-timeout 10s \
        --restart unless-stopped \
        -e MAX_CONCURRENT_ACTIONS=${MAX_CONCURRENT_ACTIONS:-5} \
        action_executor:${ACTION_EXECUTOR_VERSION:-latest}
    check_health action_executor "http://localhost:8082/health" 60
}

# Function to deploy Code Execution Engine
deploy_code_execution_engine() {
    echo "Deploying Code Execution Engine..."
    docker run -d --name code_execution_engine --network devlm-network \
        --cpus ${CODE_EXECUTION_CPUS:-2} --memory ${CODE_EXECUTION_MEMORY:-4g} --pids-limit ${CODE_EXECUTION_PIDS:-100} \
        --cap-drop ALL --security-opt no-new-privileges \
        -v ${LANGUAGE_RUNTIMES_PATH}:/runtimes:ro \
        --health-cmd "curl -f http://localhost:8083/health || exit 1" \
        --health-interval 30s \
        --health-retries 3 \
        --health-timeout 10s \
        --restart unless-stopped \
        -e SUPPORTED_LANGUAGES=${SUPPORTED_LANGUAGES:-python,javascript,go} \
        code_execution_engine:${CODE_EXECUTION_VERSION:-latest}
    check_health code_execution_engine "http://localhost:8083/health" 60
}

# Function to deploy Redis
deploy_redis() {
    echo "Deploying Redis..."
    docker run -d --name redis -p "${REDIS_PORT:-6379}:6379" --network devlm-network \
        -v ${REDIS_DATA_PATH}:/data \
        redis:${REDIS_VERSION:-latest} redis-server \
        --appendonly yes \
        --requirepass "${REDIS_PASSWORD}" \
        --maxmemory ${REDIS_MAX_MEMORY:-1gb} \
        --maxmemory-policy ${REDIS_EVICTION_POLICY:-allkeys-lru} \
        --save 900 1 --save 300 10 --save 60 10000
    check_deployment_status redis
}

# Function to deploy PostgreSQL
deploy_postgresql() {
    echo "Deploying PostgreSQL..."
    docker run -d --name postgres -p "${POSTGRES_PORT:-5432}:5432" --network devlm-network \
        -e POSTGRES_PASSWORD=${POSTGRES_PASSWORD} \
        -e POSTGRES_USER=${POSTGRES_USER:-devlm} \
        -e POSTGRES_DB=${POSTGRES_DB:-devlm} \
        -v ${POSTGRES_DATA_PATH}:/var/lib/postgresql/data \
        -v ${POSTGRES_INIT_SCRIPTS_PATH}:/docker-entrypoint-initdb.d \
        postgres:${POSTGRES_VERSION:-latest} \
        -c max_connections=${POSTGRES_MAX_CONNECTIONS:-100} \
        -c shared_buffers=${POSTGRES_SHARED_BUFFERS:-256MB} \
        -c work_mem=${POSTGRES_WORK_MEM:-4MB} \
        -c maintenance_work_mem=${POSTGRES_MAINTENANCE_WORK_MEM:-64MB} \
        -c effective_cache_size=${POSTGRES_EFFECTIVE_CACHE_SIZE:-1GB}
    check_deployment_status postgres
}

# Function to check deployment status
check_deployment_status() {
    local container_name=$1
    local max_retries=${2:-10}
    local retry_interval=${3:-5}

    for ((i=1; i<=max_retries; i++)); do
        if docker ps | grep -q $container_name; then
            if [ "$(docker inspect -f {{.State.Health.Status}} $container_name)" == "healthy" ]; then
                echo "$container_name is running and healthy."
                return 0
            fi
        fi
        echo "Waiting for $container_name to start and become healthy... (Attempt $i/$max_retries)"
        sleep $retry_interval
    done

    echo "Error: $container_name failed to start or become healthy."
    docker logs $container_name
    return 1
}

# Function to check health endpoint
check_health() {
    local service_name=$1
    local health_endpoint=$2
    local timeout=$3

    echo "Checking health of $service_name..."
    for ((i=1; i<=timeout; i++)); do
        if curl -s -o /dev/null -w "%{http_code}" $health_endpoint | grep -q "200"; then
            echo "$service_name is healthy."
            return 0
        else
            echo "Waiting for $service_name to become healthy... (Attempt $i/$timeout)"
            sleep 1
        fi
    done

    echo "Error: $service_name is not healthy."
    docker logs $service_name
    return 1
}

# Function to create Docker network
create_docker_network() {
    if ! docker network inspect devlm-network >/dev/null 2>&1; then
        echo "Creating Docker network: devlm-network"
        docker network create --driver bridge --subnet ${DOCKER_NETWORK_SUBNET:-172.18.0.0/16} devlm-network
    else
        echo "Docker network devlm-network already exists"
    fi
}

# Function to clean up existing containers
cleanup_existing_containers() {
    local containers=("api_gateway" "llm_service" "action_executor" "code_execution_engine" "redis" "postgres")
    
    for container in "${containers[@]}"; do
        if docker ps -a | grep -q $container; then
            echo "Stopping and removing existing $container container..."
            docker stop $container
            docker rm $container
        fi
    done
}

# Function to pull latest images
pull_latest_images() {
    local images=("api_gateway" "llm_service" "action_executor" "code_execution_engine" "redis" "postgres")
    
    for image in "${images[@]}"; do
        echo "Pulling latest $image image..."
        docker pull $image:${!image_VERSION:-latest}
    done
}

# Function to check system requirements
check_system_requirements() {
    local required_memory=${REQUIRED_MEMORY:-8}
    local required_cpu=${REQUIRED_CPU:-4}
    local required_disk=${REQUIRED_DISK:-20}

    local available_memory=$(free -g | awk '/^Mem:/{print $2}')
    local available_cpu=$(nproc)
    local available_disk=$(df -BG / | awk 'NR==2 {print $4}' | sed 's/G//')

    if [ $available_memory -lt $required_memory ] || [ $available_cpu -lt $required_cpu ] || [ $available_disk -lt $required_disk ]; then
        echo "Error: Insufficient system resources."
        echo "Required: ${required_memory}GB RAM, ${required_cpu} CPU cores, ${required_disk}GB free disk space."
        echo "Available: ${available_memory}GB RAM, ${available_cpu} CPU cores, ${available_disk}GB free disk space."
        exit 1
    fi

    # Check Docker version
    local docker_version=$(docker version --format '{{.Server.Version}}')
    local required_docker_version=${REQUIRED_DOCKER_VERSION:-20.10.0}
    if ! command_exists docker || ! [[ "$(printf '%s\n' "$required_docker_version" "$docker_version" | sort -V | head -n1)" = "$required_docker_version" ]]; then
        echo "Error: Docker version $required_docker_version or higher is required."
        echo "Current version: $docker_version"
        exit 1
    fi
}

# Function to configure environment variables
configure_environment() {
    if [ ! -f .env ]; then
        echo "Creating .env file..."
        cat << EOF > .env
LLM_API_KEY=<YOUR_LLM_API_KEY>
LLM_MODEL=<YOUR_LLM_MODEL>
FALLBACK_MODEL=<YOUR_FALLBACK_MODEL>
POSTGRES_PASSWORD=<POSTGRES_PASSWORD>
POSTGRES_USER=devlm
POSTGRES_DB=devlm
REDIS_PASSWORD=<REDIS_PASSWORD>
PROJECT_PATH=<PATH_TO_PROJECT>
REDIS_DATA_PATH=<PATH_TO_REDIS_DATA>
POSTGRES_DATA_PATH=<PATH_TO_POSTGRES_DATA>
POSTGRES_INIT_SCRIPTS_PATH=<PATH_TO_POSTGRES_INIT_SCRIPTS>
LANGUAGE_RUNTIMES_PATH=<PATH_TO_LANGUAGE_RUNTIMES>
API_GATEWAY_VERSION=latest
LLM_SERVICE_VERSION=latest
ACTION_EXECUTOR_VERSION=latest
CODE_EXECUTION_VERSION=latest
REDIS_VERSION=latest
POSTGRES_VERSION=latest
REQUIRED_MEMORY=8
REQUIRED_CPU=4
REQUIRED_DISK=20
REQUIRED_DOCKER_VERSION=20.10.0
MAX_TOKENS=4096
TEMPERATURE=0.7
MAX_CONCURRENT_ACTIONS=5
SUPPORTED_LANGUAGES=python,javascript,go
API_GATEWAY_LOG_LEVEL=info
EOF
        echo ".env file created. Please update with your actual values."
        exit 1
    else
        echo "Loading environment variables from .env file..."
    fi
    
    # Load environment variables
    set -a
    source .env
    set +a
}

# Function to perform rolling update
perform_rolling_update() {
    local service=$1
    local new_image=$2

    echo "Performing rolling update for $service..."
    docker service update --image $new_image --update-parallelism 1 --update-delay 30s $service
}

# Function to backup data before deployment
backup_data() {
    local backup_dir="${BACKUP_DIR:-/tmp/devlm_backup}"
    local timestamp=$(date +%Y%m%d_%H%M%S)
    
    mkdir -p "$backup_dir"
    
    echo "Backing up Redis data..."
    docker run --rm -v ${REDIS_DATA_PATH}:/data -v ${backup_dir}:/backup redis:${REDIS_VERSION:-latest} \
        sh -c "redis-cli SAVE && cp /data/dump.rdb /backup/redis_${timestamp}.rdb"
    
    echo "Backing up PostgreSQL data..."
    docker exec postgres pg_dumpall -c -U ${POSTGRES_USER:-devlm} > "${backup_dir}/postgres_${timestamp}.sql"
    
    echo "Backup completed. Files are stored in ${backup_dir}"
}

# Function to restore data from backup
restore_data() {
    local backup_dir="${BACKUP_DIR:-/tmp/devlm_backup}"
    local redis_backup=$(ls -t ${backup_dir}/redis_*.rdb | head -n1)
    local postgres_backup=$(ls -t ${backup_dir}/postgres_*.sql | head -n1)
    
    if [ -f "$redis_backup" ]; then
        echo "Restoring Redis data from ${redis_backup}..."
        docker run --rm -v ${REDIS_DATA_PATH}:/data -v ${backup_dir}:/backup redis:${REDIS_VERSION:-latest} \
            sh -c "cp /backup/$(basename $redis_backup) /data/dump.rdb"
    else
        echo "No Redis backup found. Skipping restore."
    fi
    
    if [ -f "$postgres_backup" ]; then
        echo "Restoring PostgreSQL data from ${postgres_backup}..."
        cat "$postgres_backup" | docker exec -i postgres psql -U ${POSTGRES_USER:-devlm}
    else
        echo "No PostgreSQL backup found. Skipping restore."
    fi
}

# Main deployment function
main() {
    if ! command_exists docker; then
        echo "Error: Docker is not installed. Please install Docker before running this script."
        exit 1
    fi

    configure_environment
    check_system_requirements
    backup_data
    cleanup_existing_containers
    pull_latest_images
    create_docker_network

    deploy_redis
    deploy_postgresql
    deploy_api_gateway
    deploy_llm_service
    deploy_action_executor
    deploy_code_execution_engine

    echo "Deployment completed successfully!"
}

# Run the main function
main