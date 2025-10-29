# LogAnalyzer Kubernetes Deployment Script
# Requires PowerShell 5.1+ and kubectl

param(
    [switch]$SkipIngress,
    [switch]$Force
)

# Colors for output
$Green = "Green"
$Yellow = "Yellow"
$Red = "Red"

function Write-Status {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor $Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor $Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor $Red
}

Write-Status "🚀 Deploying LogAnalyzer to Kubernetes..."

# Check if kubectl is available
try {
    $null = Get-Command kubectl -ErrorAction Stop
} catch {
    Write-Error "kubectl is not installed or not in PATH"
    exit 1
}

# Check if we're connected to a cluster
try {
    kubectl cluster-info | Out-Null
} catch {
    Write-Error "Not connected to a Kubernetes cluster"
    exit 1
}

$context = kubectl config current-context
Write-Status "Connected to Kubernetes cluster: $context"

# Create namespace
Write-Status "Creating namespace..."
kubectl apply -f namespace.yaml

# Deploy RBAC
Write-Status "Deploying RBAC..."
kubectl apply -f rbac.yaml

# Create persistent volumes
Write-Status "Creating persistent volume claims..."
kubectl apply -f persistent-volume-claims.yaml

# Check if secrets exist, if not create them
$secretExists = kubectl get secret elasticsearch-secret -n loganalyzer 2>$null
if (-not $secretExists) {
    Write-Warning "Elasticsearch secret not found. Creating with default values..."
    Write-Warning "Please update the secret with your actual Elasticsearch credentials!"
    kubectl apply -f secrets.yaml
} else {
    Write-Status "Elasticsearch secret already exists"
}

# Deploy ConfigMap
Write-Status "Deploying configuration..."
kubectl apply -f configmap.yaml

# Deploy application
Write-Status "Deploying LogAnalyzer application..."
kubectl apply -f deployment.yaml

# Deploy service
Write-Status "Creating service..."
kubectl apply -f service.yaml

# Deploy ingress (optional)
if (-not $SkipIngress) {
    $deployIngress = if ($Force) { $true } else {
        $response = Read-Host "Do you want to deploy the ingress? (y/N)"
        $response -match "^[Yy]"
    }

    if ($deployIngress) {
        Write-Warning "Please update ingress.yaml with your actual domain before deploying!"
        kubectl apply -f ingress.yaml
    }
}

# Wait for deployment to be ready
Write-Status "Waiting for deployment to be ready..."
try {
    kubectl wait --for=condition=available --timeout=300s deployment/loganalyzer -n loganalyzer
} catch {
    Write-Error "Deployment failed to become ready within 5 minutes"
    exit 1
}

# Show status
Write-Status "Deployment completed successfully!"
Write-Host ""
Write-Host "📊 Status:"
kubectl get pods -n loganalyzer
kubectl get services -n loganalyzer
kubectl get ingress -n loganalyzer

Write-Host ""
Write-Status "LogAnalyzer is now running!"

# Get service IP
$serviceIP = kubectl get svc loganalyzer-service -n loganalyzer -o jsonpath='{.spec.clusterIP}'

Write-Host "🌐 Service URLs:"
Write-Host "  - HTTP Input: http://$($serviceIP):8080/logs"
Write-Host "  - Metrics: http://$($serviceIP):9091/metrics"
Write-Host ""
Write-Host "📝 Next steps:"
Write-Host "  1. Update Elasticsearch credentials in secrets.yaml"
Write-Host "  2. Configure ingress with your domain if needed"
Write-Host "  3. Check logs: kubectl logs -f deployment/loganalyzer -n loganalyzer"
Write-Host "  4. Monitor metrics at the /metrics endpoint"