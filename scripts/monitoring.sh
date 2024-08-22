#!/bin/bash

# Environment variables (replace with actual values or use env vars)
export DB_NAME="devlm_db"
export DB_USER="devlm_user"
export DB_PASSWORD="your_db_password"
export API_GATEWAY_PORT="8080"
export LLM_SERVICE_PORT="5000"
export ACTION_EXECUTOR_PORT="7000"
export CODE_EXECUTION_PORT="6000"
export NETWORK_INTERFACE="eth0"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to monitor CPU usage
monitor_cpu() {
    if command_exists top; then
        top -bn1 | grep "Cpu(s)" | sed "s/.*, *\([0-9.]*\)%* id.*/\1/" | awk '{print 100 - $1"%"}'
    elif command_exists mpstat; then
        mpstat 1 1 | awk '$12 ~ /[0-9.]+/ {print 100 - $12"%"}'
    elif command_exists vmstat; then
        vmstat 1 2 | tail -1 | awk '{print 100 - $15"%"}'
    else
        echo "Error: No suitable command found for CPU monitoring"
    fi
}

# Function to monitor memory usage
monitor_memory() {
    if command_exists free; then
        free -m | awk 'NR==2{printf "%.2f%%", $3*100/$2 }'
    elif [ -f /proc/meminfo ]; then
        awk '/MemTotal:|MemAvailable:/ {print $2}' /proc/meminfo | paste -sd- | bc | awk '{printf "%.2f%%", $1/('$(awk '/MemTotal:/ {print $2}' /proc/meminfo)')*100}'
    elif command_exists vm_stat; then
        vm_stat | awk '/Pages active/ {active=$3} /Pages wired/ {wired=$3} /Pages free/ {free=$3} END {total=active+wired+free; printf "%.2f%%", (active+wired)/total*100}'
    else
        echo "Error: Unable to retrieve memory usage"
    fi
}

# Function to monitor disk usage
monitor_disk() {
    if command_exists df; then
        df -h / | awk 'NR==2 {print $5}'
    elif command_exists diskutil; then
        diskutil info / | awk '/Percentage Used:/ {print $3}'
    else
        echo "Error: Unable to retrieve disk usage"
    fi
}

# Function to monitor Docker containers
monitor_docker() {
    if command_exists docker; then
        docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
    else
        echo "Error: Docker is not installed or running"
    fi
}

# Function to monitor PostgreSQL
monitor_postgresql() {
    if command_exists psql; then
        PGPASSWORD=${DB_PASSWORD} psql -h localhost -U ${DB_USER} -d ${DB_NAME} -c "
            SELECT datname, numbackends, xact_commit, xact_rollback, blks_read, blks_hit,
                   tup_returned, tup_fetched, tup_inserted, tup_updated, tup_deleted,
                   pg_size_pretty(pg_database_size('${DB_NAME}')) as db_size
            FROM pg_stat_database WHERE datname = '${DB_NAME}';"
    else
        echo "Error: PostgreSQL is not installed or psql command not found"
    fi
}

# Function to monitor Redis
monitor_redis() {
    if command_exists redis-cli; then
        redis-cli info | grep -E "used_memory|used_memory_peak|connected_clients|total_connections_received|total_commands_processed|keyspace_hits|keyspace_misses|maxmemory|maxmemory_policy"
    else
        echo "Error: Redis is not installed or redis-cli command not found"
    fi
}

# Function to monitor network usage
monitor_network() {
    if command_exists ifstat; then
        ifstat -i ${NETWORK_INTERFACE:-eth0} -q 1 1 | awk 'NR==3 {print "IN: " $1 " KB/s, OUT: " $2 " KB/s"}'
    elif command_exists sar; then
        sar -n DEV 1 1 | awk '$2 == "'${NETWORK_INTERFACE:-eth0}'" {print "IN: " $5 " KB/s, OUT: " $6 " KB/s"}'
    elif command_exists netstat; then
        netstat -I ${NETWORK_INTERFACE:-en0} -b | awk 'NR==2 {print "IN: " $7 " bytes, OUT: " $10 " bytes"}'
    else
        echo "Error: No suitable command found for network monitoring"
    fi
}

# Function to monitor system load
monitor_load() {
    if [ -f /proc/loadavg ]; then
        cat /proc/loadavg | awk '{print $1", "$2", "$3}'
    elif command_exists uptime; then
        uptime | awk -F'[a-z]:' '{ print $2}' | awk -F',' '{print $1", "$2", "$3}'
    elif command_exists sysctl; then
        sysctl -n vm.loadavg | awk '{print $2", "$3", "$4}'
    else
        echo "Error: Unable to read system load"
    fi
}

# Function to monitor API Gateway
monitor_api_gateway() {
    if command_exists curl; then
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${API_GATEWAY_PORT:-8080}/health)
        case $response in
            200) echo "Healthy" ;;
            *) echo "Unhealthy (HTTP $response)" ;;
        esac
    else
        echo "Error: 'curl' command not found"
    fi
}

# Function to monitor LLM Service
monitor_llm_service() {
    if command_exists curl; then
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${LLM_SERVICE_PORT:-5000}/health)
        case $response in
            200) echo "Healthy" ;;
            *) echo "Unhealthy (HTTP $response)" ;;
        esac
    else
        echo "Error: 'curl' command not found"
    fi
}

# Function to monitor Action Executor
monitor_action_executor() {
    if command_exists curl; then
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${ACTION_EXECUTOR_PORT:-7000}/health)
        case $response in
            200) echo "Healthy" ;;
            *) echo "Unhealthy (HTTP $response)" ;;
        esac
    else
        echo "Error: 'curl' command not found"
    fi
}

# Function to monitor Code Execution Engine
monitor_code_execution_engine() {
    if command_exists curl; then
        response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${CODE_EXECUTION_PORT:-6000}/health)
        case $response in
            200) echo "Healthy" ;;
            *) echo "Unhealthy (HTTP $response)" ;;
        esac
    else
        echo "Error: 'curl' command not found"
    fi
}

# Function to log monitoring results
log_monitoring_results() {
    local log_file="${MONITORING_LOG_FILE:-/var/log/devlm_monitoring.log}"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    echo "$timestamp - $1" >> "$log_file"
    
    # Rotate log file if it exceeds 10MB
    if [ -f "$log_file" ] && [ $(du -k "$log_file" | cut -f1) -gt 10240 ]; then
        mv "$log_file" "${log_file}.1"
        touch "$log_file"
        gzip "${log_file}.1"
    fi
}

# Function to check resource thresholds
check_thresholds() {
    local cpu_usage=$(monitor_cpu | sed 's/%//')
    local memory_usage=$(monitor_memory | sed 's/%//')
    local disk_usage=$(monitor_disk | sed 's/%//')
    local load_avg=$(monitor_load | cut -d',' -f1)

    local cpu_critical=${CPU_CRITICAL:-90}
    local cpu_warning=${CPU_WARNING:-80}
    local memory_critical=${MEMORY_CRITICAL:-90}
    local memory_warning=${MEMORY_WARNING:-80}
    local disk_critical=${DISK_CRITICAL:-90}
    local disk_warning=${DISK_WARNING:-80}
    local load_critical=${LOAD_CRITICAL:-10}
    local load_warning=${LOAD_WARNING:-5}

    if (( $(echo "$cpu_usage > $cpu_critical" | bc -l) )); then
        log_monitoring_results "CRITICAL: High CPU usage - $cpu_usage%"
    elif (( $(echo "$cpu_usage > $cpu_warning" | bc -l) )); then
        log_monitoring_results "WARNING: High CPU usage - $cpu_usage%"
    fi

    if (( $(echo "$memory_usage > $memory_critical" | bc -l) )); then
        log_monitoring_results "CRITICAL: High memory usage - $memory_usage%"
    elif (( $(echo "$memory_usage > $memory_warning" | bc -l) )); then
        log_monitoring_results "WARNING: High memory usage - $memory_usage%"
    fi

    if (( $(echo "$disk_usage > $disk_critical" | bc -l) )); then
        log_monitoring_results "CRITICAL: High disk usage - $disk_usage%"
    elif (( $(echo "$disk_usage > $disk_warning" | bc -l) )); then
        log_monitoring_results "WARNING: High disk usage - $disk_usage%"
    fi

    if (( $(echo "$load_avg > $load_critical" | bc -l) )); then
        log_monitoring_results "CRITICAL: High system load - $load_avg"
    elif (( $(echo "$load_avg > $load_warning" | bc -l) )); then
        log_monitoring_results "WARNING: High system load - $load_avg"
    fi
}

# Function to format output as JSON
format_json_output() {
    local cpu_usage=$(monitor_cpu)
    local memory_usage=$(monitor_memory)
    local disk_usage=$(monitor_disk)
    local network_usage=$(monitor_network)
    local system_load=$(monitor_load)
    local api_gateway_status=$(monitor_api_gateway)
    local llm_service_status=$(monitor_llm_service)
    local action_executor_status=$(monitor_action_executor)
    local code_execution_engine_status=$(monitor_code_execution_engine)

    jq -n \
    --arg cpu "$cpu_usage" \
    --arg mem "$memory_usage" \
    --arg disk "$disk_usage" \
    --arg net "$network_usage" \
    --arg load "$system_load" \
    --arg api "$api_gateway_status" \
    --arg llm "$llm_service_status" \
    --arg action "$action_executor_status" \
    --arg code "$code_execution_engine_status" \
    '{
        cpu_usage: $cpu,
        memory_usage: $mem,
        disk_usage: $disk,
        network_usage: $net,
        system_load: $load,
        api_gateway_status: $api,
        llm_service_status: $llm,
        action_executor_status: $action,
        code_execution_engine_status: $code
    }'
}

# Main monitoring function
main() {
    local output_format=${1:-"text"}

    if [ "$output_format" == "json" ]; then
        format_json_output
    else
        echo "DevLM Monitoring"
        echo "----------------"
        echo "CPU Usage: $(monitor_cpu)"
        echo "Memory Usage: $(monitor_memory)"
        echo "Disk Usage: $(monitor_disk)"
        echo "Network Usage: $(monitor_network)"
        echo "System Load: $(monitor_load)"
        echo ""
        echo "Docker Containers:"
        monitor_docker
        echo ""
        echo "PostgreSQL Stats:"
        monitor_postgresql
        echo ""
        echo "Redis Stats:"
        monitor_redis
        echo ""
        echo "API Gateway Status: $(monitor_api_gateway)"
        echo "LLM Service Status: $(monitor_llm_service)"
        echo "Action Executor Status: $(monitor_action_executor)"
        echo "Code Execution Engine Status: $(monitor_code_execution_engine)"
    fi

    check_thresholds
}

# Run the main monitoring function
main "$@"