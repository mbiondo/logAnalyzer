#!/bin/bash

# LogAnalyzer - Quick Start Script
# Run this script to start the complete example environment

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

echo -e "${CYAN}===============================================${NC}"
echo -e "${CYAN}  LogAnalyzer - Complete Example Setup       ${NC}"
echo -e "${CYAN}===============================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
EXAMPLES_PATH="$SCRIPT_DIR/examples"

if [ ! -d "$EXAMPLES_PATH" ]; then
    echo -e "${RED}Error: examples/ directory not found!${NC}"
    exit 1
fi

echo -e "${YELLOW}Navigating to examples directory...${NC}"
cd "$EXAMPLES_PATH"

echo -e "${YELLOW}Starting all services with Docker Compose...${NC}"
docker-compose up -d

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}Services started successfully!${NC}"
    echo ""
    echo -e "${GREEN}===============================================${NC}"
    echo -e "${GREEN}           Access Services                     ${NC}"
    echo -e "${GREEN}===============================================${NC}"
    echo -e "${WHITE}Grafana (admin/admin): http://localhost:3000${NC}"
    echo -e "${WHITE}Kibana:                http://localhost:5601${NC}"
    echo -e "${WHITE}Prometheus:            http://localhost:9090${NC}"
    echo -e "${WHITE}Elasticsearch:         http://localhost:9200${NC}"
    echo -e "${WHITE}LogAnalyzer HTTP:      http://localhost:8080${NC}"
    echo -e "${WHITE}LogAnalyzer Metrics:   http://localhost:9091/metrics${NC}"
    echo -e "${GREEN}===============================================${NC}"
    echo ""
    echo -e "${YELLOW}Services status:${NC}"
    docker-compose ps
    echo ""
    echo -e "${CYAN}Useful commands:${NC}"
    echo -e "${WHITE}  View logs:         docker logs loganalyzer-service -f${NC}"
    echo -e "${WHITE}  Stop services:     docker-compose down${NC}"
    echo -e "${WHITE}  Restart:           docker-compose restart${NC}"
    echo -e "${WHITE}  Clean volumes:     docker-compose down -v${NC}"
    echo ""
    echo -e "${CYAN}See examples/README.md for detailed usage guide${NC}"
    echo ""
else
    echo ""
    echo -e "${RED}Failed to start services!${NC}"
    echo -e "${YELLOW}Check the error messages above for details.${NC}"
    exit 1
fi
