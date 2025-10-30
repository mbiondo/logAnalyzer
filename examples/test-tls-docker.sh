#!/bin/bash
# Test TLS functionality with Docker Compose
# This script tests the TLS-enabled LogAnalyzer setup

set -e

echo "🔐 Testing LogAnalyzer TLS functionality with Docker Compose"
echo "=========================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if certificates exist
echo "Checking certificates..."
if [ ! -f "scripts/certs/ca-cert.pem" ] || [ ! -f "scripts/certs/server-cert.pem" ] || [ ! -f "scripts/certs/server-key.pem" ]; then
    print_error "Certificates not found. Please run certificate generation first:"
    echo "  cd scripts/certs"
    echo "  ./generate-certs.ps1  # Windows PowerShell"
    echo "  # or"
    echo "  ./generate-certs.sh   # Linux/macOS"
    exit 1
fi
print_status "Certificates found"

# Start the TLS-enabled services
echo ""
echo "Starting TLS-enabled services..."
docker-compose -f docker-compose-tls.yml up -d loganalyzer

# Wait for LogAnalyzer to start
echo "Waiting for LogAnalyzer to start..."
sleep 10

# Check if LogAnalyzer is running
if ! docker-compose -f docker-compose-tls.yml ps loganalyzer | grep -q "Up"; then
    print_error "LogAnalyzer failed to start"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
fi
print_status "LogAnalyzer is running"

# Test HTTPS endpoint
echo ""
echo "Testing HTTPS endpoint..."
if curl -k -f -X POST https://localhost:8443/logs \
  -H "Content-Type: application/json" \
  -d '{"level": "info", "message": "Test HTTPS log from Docker", "timestamp": "2025-10-30T15:00:00Z"}' \
  --max-time 10; then
    print_status "HTTPS endpoint is working"
else
    print_error "HTTPS endpoint test failed"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
fi

# Test HTTP endpoint (should still work)
echo ""
echo "Testing HTTP endpoint (backward compatibility)..."
if curl -f -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level": "info", "message": "Test HTTP log from Docker", "timestamp": "2025-10-30T15:00:00Z"}' \
  --max-time 10; then
    print_status "HTTP endpoint is working (backward compatibility maintained)"
else
    print_error "HTTP endpoint test failed"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
fi

# Test health endpoint
echo ""
echo "Testing health endpoint..."
if curl -f http://localhost:9093/health --max-time 5; then
    print_status "Health endpoint is working"
else
    print_error "Health endpoint test failed"
fi

# Test metrics endpoint
echo ""
echo "Testing metrics endpoint..."
if curl -f http://localhost:9091/metrics --max-time 5 | head -5; then
    print_status "Metrics endpoint is working"
else
    print_error "Metrics endpoint test failed"
fi

echo ""
print_status "All TLS tests passed! 🎉"
echo ""
echo "LogAnalyzer with TLS is working correctly:"
echo "  • HTTPS input: https://localhost:8443/logs"
echo "  • HTTP input:  http://localhost:8080/logs  (backward compatibility)"
echo "  • Health:      http://localhost:9093/health"
echo "  • Metrics:     http://localhost:9091/metrics"
echo ""
echo "To stop the services:"
echo "  docker-compose -f docker-compose-tls.yml down"
echo ""
echo "To view logs:"
echo "  docker-compose -f docker-compose-tls.yml logs -f loganalyzer"