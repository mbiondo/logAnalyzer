# Test TLS functionality with Docker Compose
# This script tests the TLS-enabled LogAnalyzer setup

param(
    [switch]$SkipCleanup
)

Write-Host "🔐 Testing LogAnalyzer TLS functionality with Docker Compose" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Function to print colored output
function Write-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

# Check if certificates exist
Write-Host ""
Write-Host "Checking certificates..."
$certFiles = @(
    "scripts/certs/ca-cert.pem",
    "scripts/certs/server-cert.pem",
    "scripts/certs/server-key.pem"
)

$certMissing = $false
foreach ($certFile in $certFiles) {
    if (!(Test-Path $certFile)) {
        Write-Error "Certificate not found: $certFile"
        $certMissing = $true
    }
}

if ($certMissing) {
    Write-Error "Certificates not found. Please run certificate generation first:"
    Write-Host "  cd scripts/certs"
    Write-Host "  .\generate-certs.ps1"
    exit 1
}
Write-Success "Certificates found"

# Start the TLS-enabled services
Write-Host ""
Write-Host "Starting TLS-enabled services..."
docker-compose -f docker-compose-tls.yml up -d loganalyzer

# Wait for LogAnalyzer to start
Write-Host "Waiting for LogAnalyzer to start..."
Start-Sleep -Seconds 10

# Check if LogAnalyzer is running
$loganalyzerStatus = docker-compose -f docker-compose-tls.yml ps loganalyzer
if ($loganalyzerStatus -notmatch "Up") {
    Write-Error "LogAnalyzer failed to start"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
}
Write-Success "LogAnalyzer is running"

# Test HTTPS endpoint
Write-Host ""
Write-Host "Testing HTTPS endpoint..."
try {
    $response = Invoke-WebRequest -Uri "https://localhost:8443/logs" -Method POST `
        -ContentType "application/json" `
        -Body '{"level": "info", "message": "Test HTTPS log from Docker", "timestamp": "2025-10-30T15:00:00Z"}' `
        -SkipCertificateCheck `
        -TimeoutSec 10
    if ($response.StatusCode -eq 200) {
        Write-Success "HTTPS endpoint is working"
    } else {
        Write-Error "HTTPS endpoint returned status code: $($response.StatusCode)"
        throw "HTTPS test failed"
    }
} catch {
    Write-Error "HTTPS endpoint test failed: $($_.Exception.Message)"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
}

# Test HTTP endpoint (should still work)
Write-Host ""
Write-Host "Testing HTTP endpoint (backward compatibility)..."
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/logs" -Method POST `
        -ContentType "application/json" `
        -Body '{"level": "info", "message": "Test HTTP log from Docker", "timestamp": "2025-10-30T15:00:00Z"}' `
        -TimeoutSec 10
    if ($response.StatusCode -eq 200) {
        Write-Success "HTTP endpoint is working (backward compatibility maintained)"
    } else {
        Write-Error "HTTP endpoint returned status code: $($response.StatusCode)"
        throw "HTTP test failed"
    }
} catch {
    Write-Error "HTTP endpoint test failed: $($_.Exception.Message)"
    docker-compose -f docker-compose-tls.yml logs loganalyzer
    exit 1
}

# Test health endpoint
Write-Host ""
Write-Host "Testing health endpoint..."
try {
    $response = Invoke-WebRequest -Uri "http://localhost:9093/health" -TimeoutSec 5
    if ($response.StatusCode -eq 200) {
        Write-Success "Health endpoint is working"
    } else {
        Write-Warning "Health endpoint returned status code: $($response.StatusCode)"
    }
} catch {
    Write-Warning "Health endpoint test failed: $($_.Exception.Message)"
}

# Test metrics endpoint
Write-Host ""
Write-Host "Testing metrics endpoint..."
try {
    $response = Invoke-WebRequest -Uri "http://localhost:9091/metrics" -TimeoutSec 5
    if ($response.StatusCode -eq 200) {
        Write-Success "Metrics endpoint is working"
    } else {
        Write-Warning "Metrics endpoint returned status code: $($response.StatusCode)"
    }
} catch {
    Write-Warning "Metrics endpoint test failed: $($_.Exception.Message)"
}

Write-Host ""
Write-Success "All TLS tests passed! 🎉"
Write-Host ""
Write-Host "LogAnalyzer with TLS is working correctly:" -ForegroundColor Green
Write-Host "  • HTTPS input: https://localhost:8443/logs" -ForegroundColor White
Write-Host "  • HTTP input:  http://localhost:8080/logs  (backward compatibility)" -ForegroundColor White
Write-Host "  • Health:      http://localhost:9093/health" -ForegroundColor White
Write-Host "  • Metrics:     http://localhost:9091/metrics" -ForegroundColor White
Write-Host ""
Write-Host "To stop the services:" -ForegroundColor Yellow
Write-Host "  docker-compose -f docker-compose-tls.yml down"
Write-Host ""
Write-Host "To view logs:" -ForegroundColor Yellow
Write-Host "  docker-compose -f docker-compose-tls.yml logs -f loganalyzer"