# LogAnalyzer - Quick Start Script
# Run this script to start the complete example environment

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "  LogAnalyzer - Complete Example Setup       " -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host ""

$examplesPath = Join-Path $PSScriptRoot "examples"

if (-not (Test-Path $examplesPath)) {
    Write-Host "Error: examples/ directory not found!" -ForegroundColor Red
    exit 1
}

Write-Host "Navigating to examples directory..." -ForegroundColor Yellow
Set-Location $examplesPath

Write-Host "Starting all services with Docker Compose..." -ForegroundColor Yellow
docker-compose up -d

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "Services started successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "===============================================" -ForegroundColor Green
    Write-Host "           Access Services                     " -ForegroundColor Green
    Write-Host "===============================================" -ForegroundColor Green
    Write-Host "Grafana (admin/admin): http://localhost:3000" -ForegroundColor White
    Write-Host "Kibana:                http://localhost:5601" -ForegroundColor White
    Write-Host "Prometheus:            http://localhost:9090" -ForegroundColor White
    Write-Host "Elasticsearch:         http://localhost:9200" -ForegroundColor White
    Write-Host "LogAnalyzer HTTP:      http://localhost:8080" -ForegroundColor White
    Write-Host "LogAnalyzer Metrics:   http://localhost:9091/metrics" -ForegroundColor White
    Write-Host "===============================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Services status:" -ForegroundColor Yellow
    docker-compose ps
    Write-Host ""
    Write-Host "Useful commands:" -ForegroundColor Cyan
    Write-Host "  View logs:         docker logs loganalyzer-service -f" -ForegroundColor White
    Write-Host "  Stop services:     docker-compose down" -ForegroundColor White
    Write-Host "  Restart:           docker-compose restart" -ForegroundColor White
    Write-Host "  Clean volumes:     docker-compose down -v" -ForegroundColor White
    Write-Host ""
    Write-Host "See examples/README.md for detailed usage guide" -ForegroundColor Cyan
    Write-Host ""
} else {
    Write-Host ""
    Write-Host "Failed to start services!" -ForegroundColor Red
    Write-Host "Check the error messages above for details." -ForegroundColor Yellow
    exit 1
}
