#!/bin/bash

# LogAnalyzer - Build Script

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Parse arguments
TEST=false
CLEAN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--test)
            TEST=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -h|--help)
            echo "Usage: ./build.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -t, --test    Run tests before building"
            echo "  -c, --clean   Clean build artifacts before building"
            echo "  -h, --help    Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${CYAN}===============================================${NC}"
echo -e "${CYAN}     LogAnalyzer - Build Script               ${NC}"
echo -e "${CYAN}===============================================${NC}"
echo ""

if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    rm -f loganalyzer
    echo -e "${GREEN}Clean complete!${NC}"
    echo ""
fi

if [ "$TEST" = true ]; then
    echo -e "${YELLOW}Running tests...${NC}"
    go test ./...
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Tests failed!${NC}"
        exit 1
    fi
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
fi

echo -e "${YELLOW}Building LogAnalyzer...${NC}"
go build -o loganalyzer cmd/main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}Build successful!${NC}"
    echo ""
    echo -e "${CYAN}Binary created: loganalyzer${NC}"
    echo ""
    echo -e "${CYAN}Usage:${NC}"
    echo -e "${WHITE}  ./loganalyzer -config examples/loganalyzer.yaml${NC}"
    echo ""
else
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi
