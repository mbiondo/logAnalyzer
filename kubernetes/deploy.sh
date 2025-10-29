#!/bin/bash
# LogAnalyzer Kubernetes Deployment Script

set -e

echo "🚀 Deploying LogAnalyzer to Kubernetes..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we're connected to a cluster
if ! kubectl cluster-info &> /dev/null; then
    print_error "Not connected to a Kubernetes cluster"
    exit 1
fi

print_status "Connected to Kubernetes cluster: $(kubectl config current-context)"

# Create namespace
print_status "Creating namespace..."
kubectl apply -f namespace.yaml

# Deploy RBAC
print_status "Deploying RBAC..."
kubectl apply -f rbac.yaml

# Create persistent volumes
print_status "Creating persistent volume claims..."
kubectl apply -f persistent-volume-claims.yaml

# Check if secrets exist, if not create them
if ! kubectl get secret elasticsearch-secret -n loganalyzer &> /dev/null; then
    print_warning "Elasticsearch secret not found. Creating with default values..."
    print_warning "Please update the secret with your actual Elasticsearch credentials!"
    kubectl apply -f secrets.yaml
else
    print_status "Elasticsearch secret already exists"
fi

# Deploy ConfigMap
print_status "Deploying configuration..."
kubectl apply -f configmap.yaml

# Deploy application
print_status "Deploying LogAnalyzer application..."
kubectl apply -f deployment.yaml

# Deploy service
print_status "Creating service..."
kubectl apply -f service.yaml

# Deploy ingress (optional)
read -p "Do you want to deploy the ingress? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    print_warning "Please update ingress.yaml with your actual domain before deploying!"
    kubectl apply -f ingress.yaml
fi

# Wait for deployment to be ready
print_status "Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/loganalyzer -n loganalyzer

# Show status
print_status "Deployment completed successfully!"
echo ""
echo "📊 Status:"
kubectl get pods -n loganalyzer
kubectl get services -n loganalyzer
kubectl get ingress -n loganalyzer

echo ""
print_status "LogAnalyzer is now running!"
echo "🌐 Service URLs:"
echo "  - HTTP Input: http://$(kubectl get svc loganalyzer-service -n loganalyzer -o jsonpath='{.spec.clusterIP}'):8080/logs"
echo "  - Metrics: http://$(kubectl get svc loganalyzer-service -n loganalyzer -o jsonpath='{.spec.clusterIP}'):9091/metrics"
echo ""
echo "📝 Next steps:"
echo "  1. Update Elasticsearch credentials in secrets.yaml"
echo "  2. Configure ingress with your domain if needed"
echo "  3. Check logs: kubectl logs -f deployment/loganalyzer -n loganalyzer"
echo "  4. Monitor metrics at the /metrics endpoint"