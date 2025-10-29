# LogAnalyzer Kubernetes Deployment

This directory contains production-ready Kubernetes manifests for deploying LogAnalyzer in a Kubernetes cluster.

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl configured to access your cluster
- Storage class configured (default: `standard`)
- Elasticsearch cluster (optional, for log storage)
- Ingress controller (nginx, traefik, etc.)

### Deploy LogAnalyzer

1. **Create the namespace:**
   ```bash
   kubectl apply -f namespace.yaml
   ```

2. **Deploy RBAC (if needed):**
   ```bash
   kubectl apply -f rbac.yaml
   ```

3. **Create persistent volumes:**
   ```bash
   kubectl apply -f persistent-volume-claims.yaml
   ```

4. **Create secrets (update with your values):**
   ```bash
   # Edit secrets.yaml with your Elasticsearch credentials
   kubectl apply -f secrets.yaml
   ```

5. **Deploy the application:**
   ```bash
   kubectl apply -f configmap.yaml
   kubectl apply -f deployment.yaml
   kubectl apply -f service.yaml
   ```

6. **Deploy ingress (optional, for external access):**
   ```bash
   # Edit ingress.yaml with your domain
   kubectl apply -f ingress.yaml
   ```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n loganalyzer

# Check service
kubectl get services -n loganalyzer

# Check ingress
kubectl get ingress -n loganalyzer

# View logs
kubectl logs -f deployment/loganalyzer -n loganalyzer

# Check metrics endpoint
kubectl port-forward svc/loganalyzer-service 9091:9091 -n loganalyzer
curl http://localhost:9091/metrics
```

## 📋 Configuration

### Environment Variables

The deployment uses the following environment variables:

- `ELASTICSEARCH_USERNAME`: Elasticsearch username (from secret)
- `ELASTICSEARCH_PASSWORD`: Elasticsearch password (from secret)

### ConfigMap

The main configuration is stored in `configmap.yaml`. Key settings:

- **Persistence**: WAL enabled with 7-day retention
- **Output Buffering**: Enabled with DLQ for failed deliveries
- **Inputs**: HTTP endpoint (port 8080) and Docker container monitoring
- **Outputs**: Elasticsearch, console, Prometheus metrics, and file archive

### Storage

The deployment creates 4 PVCs:

- `loganalyzer-wal`: Write-Ahead Logging (10Gi)
- `loganalyzer-buffers`: Output buffers (5Gi)
- `loganalyzer-dlq`: Dead Letter Queue (5Gi)
- `loganalyzer-archive`: Log archive (10Gi)

## 🔧 Customization

### Scaling

For high availability, you can:

1. **Horizontal Scaling**: Deploy multiple replicas (change `replicas: 1` in deployment.yaml)
2. **Multiple Deployments**: Create separate deployments for different log sources
3. **Load Balancing**: Use a LoadBalancer service type

### External Dependencies

To integrate with external services:

1. **Elasticsearch**: Update the `secrets.yaml` with your cluster credentials
2. **External Monitoring**: Update Prometheus configuration to scrape LogAnalyzer metrics

### Security

- **RBAC**: ServiceAccount with minimal permissions for pod log access
- **Security Context**: Non-root user execution
- **Network Policies**: Consider adding network policies for traffic control
- **TLS**: Ingress configured for SSL termination

## 📊 Monitoring

### Metrics

LogAnalyzer exposes Prometheus metrics on port 9091:

- `loganalyzer_logs_total{level="debug|info|warn|error"}`: Log counters by level

### Health Checks

- **Liveness Probe**: HTTP GET `/health` on port 8080
- **Readiness Probe**: HTTP GET `/health` on port 8080

### Logs

Application logs are available via:

```bash
kubectl logs -f deployment/loganalyzer -n loganalyzer
```

## 🔄 Updates

### Configuration Updates

Since hot-reload is enabled, you can update the ConfigMap and the application will automatically reload:

```bash
kubectl apply -f configmap.yaml
```

### Application Updates

To update the application image:

```bash
kubectl set image deployment/loganalyzer loganalyzer=loganalyzer:v2.0.0 -n loganalyzer
kubectl rollout status deployment/loganalyzer -n loganalyzer
```

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│                 Kubernetes Cluster              │
├─────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────┐    │
│  │         Ingress Controller             │    │
│  │  (nginx/traefik)                       │    │
│  └─────────────────┬───────────────────────┘    │
│                    │                            │
│  ┌─────────────────┴───────────────────────┐    │
│  │             Service                      │    │
│  │  loganalyzer-service:8080,9091          │    │
│  └─────────────────┬───────────────────────┘    │
│                    │                            │
│  ┌─────────────────┴───────────────────────┐    │
│  │           Deployment                     │    │
│  │  ├── Pod with LogAnalyzer container     │    │
│  │  ├── ConfigMap mounted                  │    │
│  │  ├── PVCs for persistence               │    │
│  │  └── ServiceAccount for RBAC            │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  External Dependencies:                         │
│  ├── Elasticsearch (optional)                   │
│  ├── Prometheus (optional)                      │
│  └── Docker socket (for container logs)         │
└─────────────────────────────────────────────────┘
```

## 🚨 Troubleshooting

### Common Issues

1. **Pod CrashLoopBackOff**:
   ```bash
   kubectl describe pod <pod-name> -n loganalyzer
   kubectl logs <pod-name> -n loganalyzer --previous
   ```

2. **PVC Pending**:
   ```bash
   kubectl describe pvc <pvc-name> -n loganalyzer
   # Check storage class and available PVs
   kubectl get storageclass
   kubectl get pv
   ```

3. **RBAC Issues**:
   ```bash
   kubectl auth can-i get pods --as=system:serviceaccount:loganalyzer:loganalyzer -n loganalyzer
   ```

4. **Elasticsearch Connection**:
   ```bash
   # Check secret values
   kubectl get secret elasticsearch-secret -n loganalyzer -o yaml
   # Test connectivity from pod
   kubectl exec -it <pod-name> -n loganalyzer -- curl -f http://elasticsearch:9200
   ```

### Logs and Debugging

```bash
# View application logs
kubectl logs -f deployment/loganalyzer -n loganalyzer

# Debug with temporary pod
kubectl run debug-pod --image=busybox --rm -it --restart=Never --namespace=loganalyzer
```

## 📚 Additional Resources

- [LogAnalyzer Documentation](../../README.md)
- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Prometheus Metrics Guide](../../README.md#prometheus)
- [Output Buffering Guide](../../OUTPUT_BUFFERING.md)