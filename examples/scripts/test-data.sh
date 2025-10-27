#!/bin/bash

# Test Data Generator for LogAnalyzer
# Sends test messages to Kafka and HTTP endpoints

# Default values
MESSAGE_COUNT=${1:-5}
KAFKA_BROKER=${2:-"localhost:9092"}
HTTP_ENDPOINT=${3:-"http://localhost:8080/logs"}

echo "🚀 LogAnalyzer Test Data Generator"
echo "================================="
echo "Sending $MESSAGE_COUNT messages to each endpoint..."
echo ""

# Sample log messages with different levels and realistic data
SAMPLE_LOGS=(
    '{"level":"info","message":"User login successful","user_id":12345,"action":"login","service":"auth"}'
    '{"level":"error","message":"Database connection failed","error":"timeout","service":"auth","db_host":"db.example.com"}'
    '{"level":"warn","message":"High memory usage detected","usage":85,"threshold":80,"service":"web"}'
    '{"level":"info","message":"Payment processed successfully","amount":99.99,"currency":"USD","transaction_id":"txn_123456","service":"payment"}'
    '{"level":"error","message":"API rate limit exceeded","endpoint":"/api/v1/users","client_ip":"192.168.1.100","service":"api"}'
    '{"level":"warn","message":"Disk space running low","disk_usage":92,"mount_point":"/var/log","service":"system"}'
    '{"level":"info","message":"Cache cleared successfully","cache_size":"2.3GB","operation":"clear","service":"cache"}'
    '{"level":"error","message":"External service unavailable","service_name":"notification-service","error_code":503,"retry_count":3,"service":"integration"}'
)

# Function to send message to Kafka
send_kafka_message() {
    local broker=$1
    local topic=$2
    local message=$3

    # Add timestamp to message
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local json_message=$(echo "$message" | jq --arg ts "$timestamp" '.timestamp = $ts')

    # Send to Kafka using docker exec
    echo "$json_message" | docker exec -i loganalyzer-kafka kafka-console-producer --bootstrap-server localhost:9092 --topic "$topic"

    if [ $? -eq 0 ]; then
        local msg_text=$(echo "$json_message" | jq -r '.message')
        echo "✅ Kafka: $msg_text"
        return 0
    else
        echo "❌ Kafka: Failed to send message"
        return 1
    fi
}

# Function to send message to HTTP endpoint
send_http_message() {
    local endpoint=$1
    local message=$2

    # Send HTTP POST request
    local response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$endpoint" \
        -H "Content-Type: application/json" \
        -d "$message" \
        --max-time 5)

    if [ "$response" = "200" ]; then
        local msg_text=$(echo "$message" | jq -r '.message')
        echo "✅ HTTP: $msg_text"
        return 0
    else
        echo "❌ HTTP: Unexpected status code $response"
        return 1
    fi
}

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo "❌ Error: jq is required but not installed. Please install jq first."
    echo "   Ubuntu/Debian: sudo apt-get install jq"
    echo "   CentOS/RHEL: sudo yum install jq"
    echo "   macOS: brew install jq"
    exit 1
fi

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Error: docker is required but not installed."
    exit 1
fi

# Main execution
success_count=0
total_messages=$((MESSAGE_COUNT * 2))  # Kafka + HTTP

for ((i=1; i<=MESSAGE_COUNT; i++)); do
    # Select random log message
    random_index=$((RANDOM % ${#SAMPLE_LOGS[@]}))
    log_message="${SAMPLE_LOGS[$random_index]}"

    echo ""
    echo "📤 Message $i/$MESSAGE_COUNT"

    # Send to Kafka
    if send_kafka_message "$KAFKA_BROKER" "application-logs" "$log_message"; then
        ((success_count++))
    fi

    # Send to HTTP
    if send_http_message "$HTTP_ENDPOINT" "$log_message"; then
        ((success_count++))
    fi

    # Small delay between messages
    sleep 0.5
done

echo ""
echo "🎉 Test Data Generation Complete!"
echo "================================="
if [ $success_count -eq $total_messages ]; then
    echo "✅ Successful: $success_count/$total_messages messages"
else
    echo "⚠️  Successful: $success_count/$total_messages messages"
fi
echo "📊 Check Grafana dashboards to see the logs"
echo "🔍 Kafka logs: kafka-logs-{yyyy.MM.dd} index"
echo "🔍 HTTP logs: json-logs-{yyyy.MM.dd} index"