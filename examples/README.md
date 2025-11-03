# LogAnalyzer Complete Example

Complete working example with all services configured and ready to run.

## 🚀 Quick Start

### Start Everything

```bash
# From the examples directory
cd examples
docker-compose up -d

# Or use the quick start script from root:
# Linux/Mac:
../start-example.sh

# Windows:
..\start-example.ps1
```

### Check Status

```bash
docker-compose ps
```

### View Logs

```bash
# LogAnalyzer logs
docker logs loganalyzer-service -f

# Demo app logs
docker logs loganalyzer-demo-app -f
```

### Stop Everything

```bash
docker-compose down
```

### Clean Volumes (Reset)

```bash
# Stop and remove all volumes (including WAL persistence data)
docker-compose down -v

# Or keep persistence data (only remove containers)
docker-compose down
```

## 📊 Access Services

| Service | URL | Credentials | Description |
|---------|-----|-------------|-------------|
| **Grafana** | http://localhost:3000 | admin / admin | Unified dashboards & visualization |
| **Grafana Dashboard** | http://localhost:3000/d/loganalyzer-metrics | admin / admin | LogAnalyzer Metrics Dashboard |
| **Kibana** | http://localhost:5601 | - | Log search & analysis (Index: `loganalyzer-*`, `json-logs-*`, `kafka-logs-*`) |
| **Prometheus** | http://localhost:9090 | - | Metrics & targets monitoring |
| **Prometheus Targets** | http://localhost:9090/targets | - | View scrape targets status |
| **Elasticsearch** | http://localhost:9200 | - | Direct API access |
| **Elasticsearch Health** | http://localhost:9200/_cluster/health | - | Cluster health status |
| **LogAnalyzer HTTP** | http://localhost:8080/logs | - | HTTP endpoint for log ingestion |
| **LogAnalyzer Metrics API (Health)** | http://localhost:9093/health | - | REST API health check endpoint |
| **LogAnalyzer Metrics API (Metrics)** | http://localhost:9093/metrics | - | REST API metrics endpoint |
| **LogAnalyzer Metrics API (Status)** | http://localhost:9093/status | - | REST API status endpoint |
| **LogAnalyzer Prometheus** | http://localhost:9091/metrics | - | Prometheus metrics endpoint |
| **Kafka** | localhost:9092 | - | Kafka broker for log streaming |

## 🎯 What's Running

### Services

1. **Elasticsearch** (9200) - Log storage and search
2. **Kibana** (5601) - Elasticsearch visualization
3. **Prometheus** (9090) - Metrics collection
4. **Grafana** (3000) - Unified dashboards
5. **Kafka** (9092) - Log streaming and messaging
6. **LogAnalyzer** (8080, 9091, 9093) - Log processing with:
   - **Write-Ahead Logging (WAL)** - Crash recovery
   - **Output Buffering** - Retry logic with DLQ
   - **Plugin Resilience** - Auto-reconnection
   - **REST API** - Service monitoring and metrics
7. **Demo App** - Generates sample logs

### Resilience Features

LogAnalyzer is configured with **Write-Ahead Logging (WAL)** for log durability:

- 📝 **All logs persisted** before processing
- 💾 **Automatic recovery** on restart
- 🔄 **Buffer size**: 100 logs
- ⏱️ **Flush interval**: 5 seconds
- 🗂️ **Retention**: 24 hours
- 📦 **Volume**: `loganalyzer_wal` (Docker named volume)

### LogAnalyzer Pipelines

```
Demo App Logs → Docker Input ("docker-demo")
                      ↓
              ┌───────┴───────┬───────────┬────────────┐
              ↓               ↓           ↓            ↓
        Elasticsearch   Prometheus    Console    Elasticsearch
        (INFO+, filtered) (all logs)  (WARN+)   (Kafka logs)
        (loganalyzer-*)               (json-logs-*)
                                           (kafka-logs-*)
```

## 📁 Files Included

```
examples/
├── docker-compose.yml              # All services configuration
├── loganalyzer.yaml                # LogAnalyzer pipeline config
├── prometheus.yml                  # Prometheus scrape config
├── scripts/
│   ├── test-data.ps1               # PowerShell script to send test logs to Kafka & HTTP
│   └── test-data.sh                # Bash script to send test logs to Kafka & HTTP
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── datasources.yaml    # Auto-configure Prometheus & ES
│   │   └── dashboards/
│   │       └── dashboard-provider.yaml
│   └── dashboards/
│       └── loganalyzer-dashboard.json  # Pre-built metrics dashboard
└── README.md                       # This file
```

## 🔧 Configuration Details

### LogAnalyzer Pipeline (loganalyzer.yaml)

**Persistence (WAL):**
- **Enabled**: Yes - logs persisted to `/data/wal` before processing
- **Buffer**: 100 logs (automatic flush)
- **Flush interval**: 5 seconds
- **Max file size**: 100MB (auto-rotation)
- **Retention**: 24 hours
- **Sync writes**: Disabled (better performance)

**Inputs:**
- `docker-demo`: Monitors demo-app container logs
- `http-api`: HTTP endpoint on port 8080
- `kafka-logs`: Consumes from Kafka topic `application-logs`

**Output Pipelines:**
- **elasticsearch-all**: All sources, INFO+ levels, filtered by regex
- **prometheus-metrics**: Only docker-demo, no filters (all logs → metrics)
- **console-errors**: All sources, WARN/ERROR only
- **elasticsearch-json**: HTTP logs with JSON parsing to `json-logs-*`
- **elasticsearch-kafka**: Kafka logs with JSON parsing to `kafka-logs-*`

### Grafana Dashboard

Pre-configured with:
- ✅ Prometheus datasource (metrics)
- ✅ Elasticsearch datasource (logs)
- ✅ LogAnalyzer Metrics Dashboard with:
  - **Total Logs Processed**: Real-time counter with color thresholds
  - **Logs by Level**: Color-coded breakdown (DEBUG, INFO, WARN, ERROR)
  - **Log Rate (per minute)**: Time-series graph showing logs/min by level
  - **Error Rate Trend**: Graph with alert thresholds for ERROR and WARN
  - **Log Distribution**: Pie chart showing percentage by level
  - **Recent Error Logs**: Live logs panel from Elasticsearch (WARN and ERROR)

**Dashboard URL**: http://localhost:3000/d/loganalyzer-metrics/loganalyzer-metrics-dashboard

## 🧪 Testing

### Send Test Logs via HTTP

```bash
# Send INFO log (no authentication)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"Test log from HTTP"}'

# Send ERROR log (no authentication)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"error","message":"Critical error occurred"}'

# Send plain text (no authentication)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: text/plain" \
  -d "Simple text log message"

# Send with Basic authentication
curl -X POST http://localhost:8080/logs \
  -u "admin:secret123" \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"Authenticated log"}'

# Send with Bearer token
curl -X POST http://localhost:8080/logs \
  -H "Authorization: Bearer your-jwt-token-here" \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"Token authenticated log"}'

# Send with API key
curl -X POST http://localhost:8080/logs \
  -H "X-API-Key: your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"API key authenticated log"}'
```

**Authentication Notes:**
- When no authentication is configured, all requests are accepted
- Only one authentication method can be configured at a time
- Failed authentication returns HTTP 401 Unauthorized
- Authentication errors are logged with request details

### Test LogAnalyzer API

```bash
# Health check
curl http://localhost:9093/health

# Service metrics (buffer stats, etc.)
curl http://localhost:9093/metrics

# Complete service status
curl http://localhost:9093/status
```

### Send Test Logs to Both Kafka and HTTP

Use the provided test scripts to send realistic log messages to both Kafka and HTTP endpoints simultaneously:

```bash
# Windows PowerShell (requires Docker containers running)
cd scripts
.\test-data.ps1                    # Send 5 messages (default)
.\test-data.ps1 -MessageCount 10   # Send 10 messages

# Linux/Mac (requires jq and Docker containers running)
cd scripts
./test-data.sh                     # Send 5 messages (default)
./test-data.sh 10                  # Send 10 messages
```

**Test scripts include:**
- Realistic JSON log messages with different levels (INFO, WARN, ERROR)
- Automatic timestamps
- Service metadata (auth, web, payment, etc.)
- Error scenarios and user actions
- Both Kafka and HTTP endpoints tested simultaneously

### Test JSON Filter Parsing

The `elasticsearch-json` pipeline demonstrates JSON parsing. Send structured JSON logs:

```bash
# Send JSON log that will be parsed
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"{\"user\":\"alice\",\"action\":\"login\",\"timestamp\":\"2023-10-27T10:00:00Z\"}"}'

# Send nested JSON log (will be flattened)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"error","message":"{\"user\":{\"name\":\"bob\",\"id\":123},\"error\":\"connection failed\"}"}'
```

**Verify parsing in Kibana:**
1. Open http://localhost:5601 → Discover
2. Set index pattern to `json-logs-*`
3. Search for parsed fields: `user:alice` or `user_name:bob`
4. See flattened fields like `user_name`, `user_id`, `error`

### Send Test Logs via Kafka

```bash
# Send JSON logs to Kafka topic (Windows PowerShell)
cd scripts
.\test-data.ps1

# Or manually send logs
echo '{"timestamp":"2023-10-27T10:00:00Z","level":"info","message":"User login successful","user_id":12345,"action":"login"}' | docker exec -i loganalyzer-kafka kafka-console-producer.sh --bootstrap-server localhost:9092 --topic application-logs

echo '{"timestamp":"2023-10-27T10:01:00Z","level":"error","message":"Database connection failed","error":"timeout","service":"auth"}' | docker exec -i loganalyzer-kafka kafka-console-producer.sh --bootstrap-server localhost:9092 --topic application-logs
```

**Verify Kafka logs in Kibana:**
1. Open http://localhost:5601 → Discover
2. Create index pattern `kafka-logs-*`
3. Search for Kafka-specific metadata: `topic:application-logs`
4. See Kafka metadata fields like `partition`, `offset`, `key`

### Verify in Kibana

1. Open http://localhost:5601
2. Go to **Menu** → **Discover**
3. Create index pattern if not exists:
   - Click **Create data view** or **Index Patterns**
   - Index pattern: `loganalyzer-*`
   - Time field: `@timestamp`
   - Click **Create**
4. Go back to **Discover**
5. View logs with filters:
   - Search: `level:ERROR`
   - Search: `level:INFO OR level:WARN`
   - Use time range picker for specific periods

### View Metrics in Prometheus

1. Open http://localhost:9090
2. Go to **Graph** tab
3. Try these queries:
   - `loganalyzer_logs_total` - Total logs by level
   - `rate(loganalyzer_logs_total[1m])` - Log rate per second
   - `sum(loganalyzer_logs_total)` - Total logs across all levels
   - `loganalyzer_logs_total{level="error"}` - Only ERROR logs
4. Go to **Status** → **Targets** to see LogAnalyzer scrape status

### Grafana Dashboard

1. Open http://localhost:3000 (admin/admin)
2. On first login, you may be prompted to change password (skip for demo)
3. Go to **Dashboards** → Browse
4. Navigate to **LogAnalyzer** folder
5. Open **LogAnalyzer Metrics Dashboard**
6. See real-time metrics and logs with:
   - Auto-refresh every 5 seconds
   - Last 15 minutes time range (configurable)
   - All 6 panels with live data

**Direct link**: http://localhost:3000/d/loganalyzer-metrics/loganalyzer-metrics-dashboard

## 🎨 Customization

### Add More Containers to Monitor

Edit `loganalyzer.yaml`:

```yaml
inputs:
  - type: docker
    name: "all-containers"
    config:
      container_filter: 
        - "my-app-*"
        - "my-service-*"
      stream: "stdout"
```

### Change Elasticsearch Index

Edit `loganalyzer.yaml`:

```yaml
outputs:
  - type: elasticsearch
    name: "custom-index"
    config:
      index: "my-logs-{yyyy.MM.dd}"
```

### Add Slack Alerts

Edit `loganalyzer.yaml`:

```yaml
outputs:
  - type: slack
    name: "alerts"
    sources: []
    filters:
      - type: level
        config:
          levels: ["ERROR"]
    config:
      webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
      channel: "#alerts"
```

## 🐛 Troubleshooting

### Verify Persistence (WAL)

```bash
# Check WAL files inside container
docker exec loganalyzer-service ls -lh /data/wal/

# Check persistence logs
docker logs loganalyzer-service 2>&1 | grep -i "persistence\|recovery\|wal"

# Windows PowerShell
docker logs loganalyzer-service 2>&1 | Select-String "persistence|recovery|wal" -CaseSensitive:$false

# Test recovery - restart LogAnalyzer
docker-compose restart loganalyzer

# Watch recovery logs
docker logs loganalyzer-service -f
# You should see: "Found X WAL files for recovery"
# and "Recovery complete: X logs recovered from Y files"

# Check WAL volume on host
docker volume inspect loganalyzer_wal

# View WAL volume location
docker volume inspect loganalyzer_wal | grep Mountpoint

# Windows PowerShell
docker volume inspect loganalyzer_wal | Select-String "Mountpoint"
```

### Elasticsearch not healthy

```bash
# Check logs
docker logs loganalyzer-elasticsearch

# Check cluster health
curl http://localhost:9200/_cluster/health

# Windows PowerShell
Invoke-WebRequest http://localhost:9200/_cluster/health

# Restart
docker-compose restart elasticsearch

# Wait for green status
docker-compose ps
```

### LogAnalyzer not connecting

```bash
# Check if elasticsearch is ready
curl http://localhost:9200/_cluster/health

# Windows PowerShell
(Invoke-WebRequest http://localhost:9200/_cluster/health).Content | ConvertFrom-Json

# Check loganalyzer logs
docker logs loganalyzer-service -f

# Check if LogAnalyzer is healthy
docker ps | grep loganalyzer
```

### No metrics in Grafana

```bash
# Verify LogAnalyzer metrics endpoint is working
curl http://localhost:9091/metrics

# Windows PowerShell
Invoke-WebRequest http://localhost:9091/metrics

# Check Prometheus targets status
# Open http://localhost:9090/targets
# LogAnalyzer should be "UP"

# Verify Prometheus is scraping
curl http://localhost:9090/api/v1/targets

# Windows PowerShell
(Invoke-WebRequest http://localhost:9090/api/v1/targets).Content | ConvertFrom-Json

# Check Prometheus logs
docker logs loganalyzer-prometheus
```

### LogAnalyzer API not responding

```bash
# Check if API port is accessible
curl http://localhost:9093/health

# Windows PowerShell
Invoke-WebRequest http://localhost:9093/health

# Check LogAnalyzer logs for API startup
docker logs loganalyzer-service 2>&1 | grep -i "api\|server"

# Windows PowerShell
docker logs loganalyzer-service 2>&1 | Select-String "api|server" -CaseSensitive:$false

# Verify API configuration in loganalyzer.yaml
docker exec loganalyzer-service cat /config/loganalyzer.yaml | grep -A 5 "api"

# Windows PowerShell
docker exec loganalyzer-service cat /config/loganalyzer.yaml | Select-String "api" -Context 5
```

### Grafana dashboard not loading

```bash
# Check Grafana logs for provisioning errors
docker logs loganalyzer-grafana 2>&1 | grep -i "dashboard\|provision\|error"

# Windows PowerShell
docker logs loganalyzer-grafana 2>&1 | Select-String "dashboard|provision|error" -CaseSensitive:$false

# Verify dashboard file is mounted
docker exec loganalyzer-grafana ls -la /var/lib/grafana/dashboards/

# Verify datasource provisioning
docker exec loganalyzer-grafana ls -la /etc/grafana/provisioning/datasources/

# Restart Grafana to reload provisioning
docker-compose restart grafana

# Wait for Grafana to be healthy
docker ps | grep grafana
```

### No logs appearing in Elasticsearch

```bash
# Check if logs are being generated by demo app
docker logs loganalyzer-demo-app --tail 20

# Check LogAnalyzer is processing logs
docker logs loganalyzer-service --tail 50

# Check Elasticsearch indices
curl http://localhost:9200/_cat/indices?v

# Windows PowerShell
(Invoke-WebRequest http://localhost:9200/_cat/indices?v).Content

# Search for recent logs
curl -X POST http://localhost:9200/loganalyzer-*/_search?size=5

# Windows PowerShell
$body = '{"size":5,"sort":[{"@timestamp":{"order":"desc"}}]}'; Invoke-WebRequest -Uri http://localhost:9200/loganalyzer-*/_search -Method POST -Body $body -ContentType "application/json"
```

### Docker compose not starting

```bash
# Check for port conflicts
# Linux/Mac
netstat -tuln | grep -E '3000|5601|9090|9200|8080|9091|9092'

# Windows PowerShell
Get-NetTCPConnection | Where-Object {$_.LocalPort -in 3000,5601,9090,9200,8080,9091,9092} | Select LocalPort,State,OwningProcess

# Check Docker is running
docker ps

# View all container statuses
docker-compose ps

# Check for errors in specific service
docker-compose logs elasticsearch
docker-compose logs kafka
docker-compose logs grafana
docker-compose logs loganalyzer
```

### Kafka not healthy

```bash
# Check Kafka logs
docker logs loganalyzer-kafka

# Test Kafka connectivity
docker exec loganalyzer-kafka kafka-broker-api-versions.sh --bootstrap-server localhost:9092

# Windows PowerShell
docker exec loganalyzer-kafka kafka-broker-api-versions.sh --bootstrap-server localhost:9092

# Check if topic exists
docker exec loganalyzer-kafka kafka-topics.sh --bootstrap-server localhost:9092 --list

# Create topic manually if needed
docker exec loganalyzer-kafka kafka-topics.sh --bootstrap-server localhost:9092 --create --topic application-logs --partitions 1 --replication-factor 1

# Restart Kafka
docker-compose restart kafka
```

## 📈 Performance Tips

### High Volume Logs

Increase Elasticsearch batch size in `loganalyzer.yaml`:

```yaml
config:
  batch_size: 100  # Default: 50
  timeout: 60      # Increase timeout
```

### Reduce Demo App Noise

Edit `docker-compose.yml`:

```yaml
demo-app:
  command: >
    sh -c "
    while true; do
      echo \"$(date) INFO Important log\";
      sleep 10;  # Increase interval
    done
    "
```

## 🔒 Security Notes

⚠️ **This is a demo configuration!** For production:

- Enable Elasticsearch security (xpack)
- Use strong Grafana passwords
- Don't expose ports directly
- Use Docker secrets for sensitive data
- Enable TLS/SSL
- Configure proper network isolation

## 🔒 TLS/MTLS Configuration

LogAnalyzer supports secure communication using TLS and Mutual TLS (MTLS) for all inputs and outputs. This section explains how to set up and test TLS configurations.

### Generate Test Certificates

First, generate test certificates for development and testing:

```bash
# Linux/Mac
cd examples
./scripts/certs/generate-certs.sh

# Windows PowerShell
cd examples
.\scripts\certs\generate-certs.ps1
```

This creates:
- `certs/ca-cert.pem` - Certificate Authority certificate
- `certs/ca-key.pem` - Certificate Authority private key
- `certs/server-cert.pem` - Server certificate for HTTPS
- `certs/server-key.pem` - Server private key
- `certs/client-cert.pem` - Client certificate for MTLS
- `certs/client-key.pem` - Client private key

### Test Certificates

Verify your certificates are valid:

```bash
# Linux/Mac
./test-certs.sh

# Windows PowerShell
.\test-certs.ps1
```

### TLS Configuration Example

Use the complete TLS example configuration:

```bash
# Start with TLS configuration
loganalyzer -config examples/loganalyzer-tls.yaml
```

The `loganalyzer-tls.yaml` includes:
- **HTTPS Input** with server certificate validation
- **Kafka Input** with TLS and optional MTLS
- **Elasticsearch Output** with TLS and optional MTLS
- **Slack Output** with custom CA support

### Test HTTPS Input

```bash
# Test basic HTTPS (skip certificate verification)
curl -k -X POST https://localhost:8443/logs \
  -H "Content-Type: application/json" \
  -d '{"message":"Test HTTPS log","level":"info"}'

# Test with client certificate (MTLS)
curl --cacert certs/ca.pem \
     --cert certs/client.pem \
     --key certs/client.key \
     -X POST https://localhost:8443/logs \
     -H "Content-Type: application/json" \
     -d '{"message":"Test MTLS log","level":"info"}'
```

### TLS Configuration Options

#### Server TLS with mTLS (HTTPS Input)
```yaml
inputs:
  - type: http
    config:
      port: "8443"
      tls:
        enabled: true
        # Server certificate validation (when this service connects to external services)
        ca_cert: "./examples/certs/ca.pem"
        insecure_skip_verify: false
        min_version: "1.2"
        max_version: "1.3"
        # Client certificate verification (for mTLS - server requires client certs)
        client_ca_cert: "./examples/certs/ca.pem"  # CA for verifying client certificates
        client_auth: "require-and-verify"          # Require and verify client certificates
      # Server certificates (required)
      cert_file: "./examples/certs/server.pem"
      key_file: "./examples/certs/server.key"
```

#### Client TLS (Outputs)
```yaml
outputs:
  - type: elasticsearch
    config:
      addresses: ["https://es.example.com:9200"]
      tls:
        enabled: true
        # Server certificate validation
        ca_cert: "./examples/certs/ca.pem"
        # Client certificate for MTLS
        client_cert: "./examples/certs/client.pem"
        client_key: "./examples/certs/client.key"
        min_version: "1.2"
        server_name: "es.example.com"
```

#### Kafka TLS
```yaml
inputs:
  - type: kafka
    config:
      brokers: ["kafka.example.com:9093"]
      tls:
        enabled: true
        ca_cert: "./examples/certs/ca.pem"
        # Optional MTLS
        client_cert: "./examples/certs/client.pem"
        client_key: "./examples/certs/client.key"
        server_name: "kafka.example.com"
```

### Certificate Files

| File | Purpose | Required For |
|------|---------|--------------|
| `ca.pem` | Certificate Authority | Server certificate validation |
| `server.pem` | Server certificate | HTTPS server |
| `server.key` | Server private key | HTTPS server |
| `client.pem` | Client certificate | mTLS authentication (client-side) |
| `client.key` | Client private key | mTLS authentication (client-side) |
| `client-ca.pem` | Client CA certificate | mTLS client verification (server-side) |

### Security Best Practices

1. **Never use test certificates in production**
2. **Use strong passwords for private keys**
3. **Restrict file permissions** (`chmod 600 *.key`)
4. **Enable certificate pinning** when possible
5. **Use short certificate lifetimes** (90 days max)
6. **Monitor certificate expiration**
7. **Use MTLS for high-security environments**

### Troubleshooting TLS

#### Certificate Errors
```bash
# Check certificate validity
openssl x509 -in certs/server.pem -text -noout

# Verify certificate chain
openssl verify -CAfile certs/ca.pem certs/server.pem

# Test server certificate
openssl s_client -connect localhost:8443 -CAfile certs/ca.pem
```

#### Common Issues
- **Certificate expired**: Regenerate certificates
- **Wrong hostname**: Check `server_name` in config
- **Permission denied**: Fix file permissions (`chmod 600 *.key`)
- **MTLS required but not provided**: Add client certificate to request

#### Debug TLS Connections
```bash
# Enable debug logging
export SSLKEYLOGFILE=/tmp/ssl.log
# Then run curl with --trace -

# Check LogAnalyzer logs for TLS errors
docker logs loganalyzer-service 2>&1 | grep -i tls
```

### Production TLS Setup

For production environments:

1. **Use proper certificates** from trusted CA
2. **Enable HSTS** headers
3. **Configure certificate rotation**
4. **Use TLS 1.3** minimum
5. **Enable OCSP stapling**
6. **Monitor certificate expiration**
7. **Use strong cipher suites**

Example production config:
```yaml
tls:
  enabled: true
  min_version: "1.3"
  max_version: "1.3"
  ca_cert: "/etc/ssl/certs/ca.pem"
  client_cert: "/etc/ssl/certs/client.pem"
  client_key: "/etc/ssl/private/client.key"
```

## 🔒 TLS/MTLS Configuration with Docker

LogAnalyzer supports secure communication using TLS and Mutual TLS (MTLS) for all inputs and outputs. This section explains how to set up and test TLS configurations with Docker Compose.

### Generate Test Certificates

First, generate test certificates for development and testing:

```bash
# Linux/Mac
cd examples
./scripts/certs/generate-certs.sh

# Windows PowerShell
cd examples
.\scripts\certs\generate-certs.ps1
```

This creates:
- `scripts/certs/ca-cert.pem` - Certificate Authority
- `scripts/certs/server-cert.pem` / `server-key.pem` - Server certificate for HTTPS
- `scripts/certs/client-cert.pem` / `client-key.pem` - Client certificate for MTLS

### Start TLS-Enabled Services

Use the TLS-enabled Docker Compose configuration:

```bash
# Start all services with TLS
docker-compose -f docker-compose-tls.yml up -d

# Or start only LogAnalyzer with TLS
docker-compose -f docker-compose-tls.yml up -d loganalyzer
```

### Test TLS Functionality

Use the provided test scripts to verify TLS is working:

```bash
# Linux/Mac
./test-tls-docker.sh

# Windows PowerShell
.\test-tls-docker.ps1
```

### TLS Configuration Details

The `docker-compose-tls.yml` and `loganalyzer-tls.yaml` include:

- **HTTPS Input** on port 8443 with server certificate validation
- **HTTP Input** on port 8080 (backward compatibility)
- **Certificate mounting** from `./scripts/certs/` to `/certs/` in container
- **Kafka Input** with TLS configuration (commented for demo)
- **Elasticsearch Output** with TLS configuration (commented for demo)

### Test HTTPS Input

```bash
# Test basic HTTPS (skip certificate verification for self-signed certs)
curl -k -X POST https://localhost:8443/logs \
  -H "Content-Type: application/json" \
  -d '{"message":"Test HTTPS log","level":"info"}'

# Test HTTP (should still work)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"message":"Test HTTP log","level":"info"}'
```

### Test Health and Metrics

```bash
# Health check
curl http://localhost:9093/health

# Prometheus metrics
curl http://localhost:9091/metrics
```

### TLS Configuration Options

#### Server TLS (HTTPS Input)
```yaml
inputs:
  - type: http
    config:
      port: "8443"
      tls:
        enabled: true
        # Server certificate validation (for client auth)
        ca_cert: "/certs/ca-cert.pem"        # CA for client verification
        insecure_skip_verify: false
        min_version: "1.2"
        max_version: "1.3"
        # Client certificate verification (for mTLS)
        client_ca_cert: "/certs/ca-cert.pem"  # CA for verifying client certificates
        client_auth: "require-and-verify"     # Require and verify client certificates
      # Server certificates (required)
      cert_file: "/certs/server-cert.pem"
      key_file: "/certs/server-key.pem"
```

#### Client TLS (Outputs)
```yaml
outputs:
  - type: elasticsearch
    config:
      addresses: ["https://elasticsearch:9200"]
      tls:
        enabled: true
        # Server certificate validation
        ca_cert: "/certs/ca-cert.pem"
        # Client certificate for MTLS
        client_cert: "/certs/client-cert.pem"
        client_key: "/certs/client-key.pem"
        min_version: "1.2"
```

### Certificate Files

| File | Purpose | Required For |
|------|---------|--------------|
| `ca-cert.pem` | Certificate Authority | Server certificate validation |
| `server-cert.pem` | Server certificate | HTTPS server |
| `server-key.pem` | Server private key | HTTPS server |
| `client-cert.pem` | Client certificate | MTLS authentication (client-side) |
| `client-key.pem` | Client private key | MTLS authentication (client-side) |
| `ca-cert.pem` | Client CA certificate | MTLS client verification (server-side) |

### Security Best Practices

1. **Never use test certificates in production**
2. **Use strong passwords for private keys**
3. **Restrict file permissions** (`chmod 600 *.key`)
4. **Enable certificate pinning** when possible
5. **Use short certificate lifetimes** (90 days max)
6. **Monitor certificate expiration**
7. **Use MTLS for high-security environments**

### Troubleshooting TLS

#### Certificate Errors
```bash
# Check certificate validity
openssl x509 -in scripts/certs/server-cert.pem -text -noout

# Verify certificate chain
openssl verify -CAfile scripts/certs/ca-cert.pem scripts/certs/server-cert.pem

# Test server certificate
openssl s_client -connect localhost:8443 -CAfile scripts/certs/ca-cert.pem
```

#### Common Issues
- **Certificate expired**: Regenerate certificates
- **Wrong hostname**: Check `server_name` in config
- **Permission denied**: Fix file permissions (`chmod 600 *.key`)
- **MTLS required but not provided**: Add client certificate to request

#### Debug TLS Connections
```bash
# Enable debug logging
export SSLKEYLOGFILE=/tmp/ssl.log
# Then run curl with --trace -

# Check LogAnalyzer logs for TLS errors
docker-compose -f docker-compose-tls.yml logs loganalyzer 2>&1 | grep -i tls
```

### Production TLS Setup

For production environments:

1. **Use proper certificates** from trusted CA
2. **Enable HSTS** headers
3. **Configure certificate rotation**
4. **Use TLS 1.3** minimum
5. **Enable OCSP stapling**
6. **Monitor certificate expiration**
7. **Use strong cipher suites**

Example production config:
```yaml
tls:
  enabled: true
  min_version: "1.3"
  max_version: "1.3"
  ca_cert: "/etc/ssl/certs/ca.pem"
  client_cert: "/etc/ssl/certs/client.pem"
  client_key: "/etc/ssl/private/client.key"
```

---

**Built with ❤️ using LogAnalyzer**
