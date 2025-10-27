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
docker-compose down -v
```

## 📊 Access Services

| Service | URL | Credentials | Description |
|---------|-----|-------------|-------------|
| **Grafana** | http://localhost:3000 | admin / admin | Unified dashboards & visualization |
| **Grafana Dashboard** | http://localhost:3000/d/loganalyzer-metrics | admin / admin | LogAnalyzer Metrics Dashboard |
| **Kibana** | http://localhost:5601 | - | Log search & analysis (Index: `loganalyzer-*`) |
| **Prometheus** | http://localhost:9090 | - | Metrics & targets monitoring |
| **Prometheus Targets** | http://localhost:9090/targets | - | View scrape targets status |
| **Elasticsearch** | http://localhost:9200 | - | Direct API access |
| **Elasticsearch Health** | http://localhost:9200/_cluster/health | - | Cluster health status |
| **LogAnalyzer HTTP** | http://localhost:8080/logs | - | HTTP endpoint for log ingestion |
| **LogAnalyzer Metrics** | http://localhost:9091/metrics | - | Prometheus metrics endpoint |

## 🎯 What's Running

### Services

1. **Elasticsearch** (9200) - Log storage and search
2. **Kibana** (5601) - Elasticsearch visualization
3. **Prometheus** (9090) - Metrics collection
4. **Grafana** (3000) - Unified dashboards
5. **LogAnalyzer** (8080, 9091) - Log processing with pipelines
6. **Demo App** - Generates sample logs

### LogAnalyzer Pipelines

```
Demo App Logs → Docker Input ("docker-demo")
                      ↓
              ┌───────┴───────┬───────────┐
              ↓               ↓           ↓
        Elasticsearch   Prometheus    Console
        (INFO+, filtered) (all logs)  (WARN+)
```

## 📁 Files Included

```
examples/
├── docker-compose.yml              # All services configuration
├── loganalyzer.yaml                # LogAnalyzer pipeline config
├── prometheus.yml                  # Prometheus scrape config
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

**Inputs:**
- `docker-demo`: Monitors demo-app container logs
- `http-api`: HTTP endpoint on port 8080

**Output Pipelines:**
- **elasticsearch-all**: All sources, INFO+ levels, filtered by regex
- **prometheus-metrics**: Only docker-demo, no filters (all logs → metrics)
- **console-errors**: All sources, WARN/ERROR only

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
# Send INFO log
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"info","message":"Test log from HTTP"}'

# Send ERROR log
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level":"error","message":"Critical error occurred"}'

# Send plain text
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: text/plain" \
  -d "Simple text log message"
```

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
input:
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
output:
  outputs:
    - type: elasticsearch
      name: "custom-index"
      config:
        index: "my-logs-{yyyy.MM.dd}"
```

### Add Slack Alerts

Edit `loganalyzer.yaml`:

```yaml
output:
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
netstat -tuln | grep -E '3000|5601|9090|9200|8080|9091'

# Windows PowerShell
Get-NetTCPConnection | Where-Object {$_.LocalPort -in 3000,5601,9090,9200,8080,9091} | Select LocalPort,State,OwningProcess

# Check Docker is running
docker ps

# View all container statuses
docker-compose ps

# Check for errors in specific service
docker-compose logs elasticsearch
docker-compose logs grafana
docker-compose logs loganalyzer
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

## 📚 Next Steps

- Review pipeline architecture in main README.md
- Customize filters for your use case
- Add more input sources (HTTP, File)
- Create custom Grafana dashboards
- Set up alerts in Grafana
- Export dashboards for backup

## 💡 Example Use Cases

### Monitor Multiple Apps

```yaml
input:
  inputs:
    - type: docker
      name: "frontend"
      config:
        container_filter: ["nginx-*", "webapp-*"]
    
    - type: docker
      name: "backend"
      config:
        container_filter: ["api-*", "worker-*"]

output:
  outputs:
    # Frontend logs → Elasticsearch
    - type: elasticsearch
      sources: ["frontend"]
      config:
        index: "frontend-{yyyy.MM.dd}"
    
    # Backend logs → Elasticsearch
    - type: elasticsearch
      sources: ["backend"]
      config:
        index: "backend-{yyyy.MM.dd}"
```

### Alert on Critical Errors

```yaml
output:
  outputs:
    - type: slack
      sources: []
      filters:
        - type: level
          config:
            levels: ["ERROR"]
        - type: regex
          config:
            patterns: ["CRITICAL", "FATAL"]
            mode: "include"
      config:
        webhook_url: "..."
```

---

**Built with ❤️ using LogAnalyzer**
