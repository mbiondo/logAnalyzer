# Output Buffering & Retry - LogAnalyzer

## Overview

LogAnalyzer includes an **Output Buffering** system that handles temporary output failures with automatic retry logic and Dead Letter Queue (DLQ) for permanently failed logs.

## Key Features

- **Automatic Retry**: Failed log deliveries are retried automatically
- **Exponential Backoff**: Retry intervals increase to avoid overwhelming failed services
- **Per-Output Queues**: Each output has its own buffer and retry queue
- **Dead Letter Queue**: Logs that fail all retries are saved for manual review
- **Persistent Buffers**: Retry queues survive application restarts
- **Zero Log Loss**: Combined with WAL, ensures no logs are lost

## How It Works

When an output fails to deliver a log:

1. **Failure Detection**: Output returns error (connection refused, timeout, etc.)
2. **Queue for Retry**: Log is added to output's retry queue
3. **Exponential Backoff**: Retries after 10s, 20s, 40s, 80s, 120s (max)
4. **Automatic Recovery**: When output recovers, queued logs are delivered
5. **DLQ Fallback**: After max retries (default: 5), log goes to DLQ file

## Configuration

```yaml
output_buffer:
  enabled: true                    # Enable/disable output buffering
  dir: "./data/buffers"           # Directory for buffer files
  max_queue_size: 1000            # Max logs in memory queue per output
  max_retries: 5                  # Max retry attempts (default: 5)
  retry_interval: 10s             # Initial retry interval (default: 10s)
  max_retry_delay: 120s           # Maximum backoff delay (default: 120s)
  flush_interval: 15s             # How often to persist retry queue
  dlq_enabled: true               # Enable Dead Letter Queue
  dlq_path: "./data/dlq"          # Path for DLQ files
```

## Configuration Options

- **`enabled`**: Enable or disable output buffering (default: `false`)
- **`dir`**: Directory for buffer state files (default: `"./data/buffers"`)
- **`max_queue_size`**: Maximum logs in memory per output (default: `1000`)
- **`max_retries`**: Number of retry attempts before sending to DLQ (default: `5`)
- **`retry_interval`**: Initial retry delay (default: `"10s"`)
- **`max_retry_delay`**: Maximum backoff delay (default: `"120s"`)
- **`flush_interval`**: How often to save retry queue to disk (default: `"15s"`)
- **`dlq_enabled`**: Enable Dead Letter Queue for failed logs (default: `true`)
- **`dlq_path`**: Directory for DLQ files (default: `"./data/dlq"`)

## Retry Timeline Example

For a log that keeps failing:
```
Attempt 1: Immediate
Attempt 2: After 10 seconds (backoff: 10s)
Attempt 3: After 20 seconds (backoff: 20s)
Attempt 4: After 40 seconds (backoff: 40s)
Attempt 5: After 80 seconds (backoff: 80s)
Attempt 6: After 120 seconds (backoff: 120s, max reached)
... continues with 120s until max_retries
→ Sent to DLQ
```

Total time for 5 retries: ~3-4 minutes

## Example Logs

When output fails and buffer activates:
```
[BUFFER:elasticsearch-all] Delivery failed: dial tcp: connection refused (attempt 1)
[BUFFER:elasticsearch-all] Retrying log (attempt 1/5, backoff: 10s)
[BUFFER:elasticsearch-all] Processing 15 logs in retry queue
[BUFFER:elasticsearch-all] Retrying log (attempt 2/5, backoff: 20s)
[BUFFER:elasticsearch-all] Retry successful!
```

When log goes to DLQ:
```
[BUFFER:elasticsearch-all] Max retries (5) reached, sending to DLQ
[BUFFER:elasticsearch-all] Log sent to DLQ: /data/dlq/elasticsearch-all-dlq.jsonl
```

## Dead Letter Queue (DLQ)

Logs that fail all retry attempts are saved to DLQ files.

### Viewing DLQ Files

```bash
# View DLQ files
ls -lh ./data/dlq/
# Output:
# elasticsearch-all-dlq.jsonl    8.5K
# slack-alerts-dlq.jsonl         2.1K

# Inspect failed logs
cat ./data/dlq/elasticsearch-all-dlq.jsonl | jq

# Count failed logs per output
wc -l ./data/dlq/*.jsonl
```

### DLQ File Format

Each DLQ file is named `{output-name}-dlq.jsonl` and contains one JSON log per line:
```json
{"timestamp":"2025-10-28T21:30:45Z","level":"error","message":"Failed to process","metadata":{"user":"alice","service":"auth"}}
{"timestamp":"2025-10-28T21:31:12Z","level":"warn","message":"Connection timeout","metadata":{"host":"api-server"}}
```

## Use Cases

### Elasticsearch Maintenance

Handle planned maintenance windows:
```yaml
output_buffer:
  enabled: true
  max_retries: 10        # Allow longer downtime (10-20 minutes)
  retry_interval: 30s    # Check every 30 seconds
  max_retry_delay: 300s  # Up to 5 minutes between retries
```

### Critical Alerts (Slack)

Ensure important alerts are never lost:
```yaml
output_buffer:
  enabled: true
  max_retries: 20        # Try harder for alerts
  retry_interval: 5s     # Retry quickly
  dlq_enabled: true      # Keep failed alerts for manual review
```

### High-Volume Processing

Handle traffic spikes:
```yaml
output_buffer:
  enabled: true
  max_queue_size: 5000   # Large buffer for traffic spikes
  flush_interval: 30s    # Less frequent disk writes (better performance)
```

## Integration with Plugin Resilience

Output buffering works seamlessly with plugin resilience:

```
Service Starts
    ↓
Plugin Unavailable → Plugin Resilience retries in background
    ↓
Service Accepts Logs (even if output not ready)
    ↓
Write Attempt → Plugin still initializing → Error
    ↓
Output Buffer → Queues log for retry
    ↓
Plugin Connects → Health check passes
    ↓
Buffer Delivers → Queued logs sent successfully
    ↓
Normal Operation → Fresh + buffered logs flow smoothly
```

## Best Practices

1. **Enable with Resilience**: Use both features for maximum reliability
2. **Size Queues Appropriately**: Set `max_queue_size` based on expected traffic and outage duration
3. **Monitor DLQ**: Set up alerts when DLQ files grow unexpectedly
4. **Tune Retries**: Adjust `max_retries` based on typical outage duration
5. **Review DLQ Periodically**: Manually investigate and reprocess important failed logs
6. **Persistent Storage**: Store buffer and DLQ directories on persistent volumes (important for Docker)
7. **Disk Space**: Monitor disk usage for buffer and DLQ directories

## Monitoring

### Check Buffer State

```bash
# List buffer state files
ls -lh ./data/buffers/
# elasticsearch-all-buffer.json
# slack-alerts-buffer.json

# View buffer state
cat ./data/buffers/elasticsearch-all-buffer.json | jq
```

### Monitor DLQ Growth

```bash
# Real-time DLQ monitoring
watch -n 5 'du -sh ./data/dlq/*'

# Count failed logs per output
wc -l ./data/dlq/*.jsonl

# View recent DLQ entries
tail -f ./data/dlq/elasticsearch-all-dlq.jsonl | jq
```

### Log Analysis

```bash
# Find buffer-related logs
docker logs loganalyzer-service | grep "BUFFER"

# Count retries
docker logs loganalyzer-service | grep "Retrying log" | wc -l

# Find DLQ writes
docker logs loganalyzer-service | grep "sent to DLQ"
```

## Docker Configuration

When using Docker, ensure volumes are configured correctly:

```yaml
services:
  loganalyzer:
    image: loganalyzer:latest
    volumes:
      - ./config.yaml:/config.yaml
      - buffer-data:/data/buffers    # Persistent buffer storage
      - dlq-data:/data/dlq            # Persistent DLQ storage
      - wal-data:/data/wal            # Persistent WAL storage

volumes:
  buffer-data:
  dlq-data:
  wal-data:
```

## Complete Reliability Stack

LogAnalyzer provides **triple protection** against log loss:

```yaml
# 1. WAL (Write-Ahead Log): Logs survive crashes
persistence:
  enabled: true
  dir: "./data/wal"
  retention_hours: 24

# 2. Output Buffering: Failed deliveries are retried
output_buffer:
  enabled: true
  dir: "./data/buffers"
  max_queue_size: 1000
  max_retries: 5
  dlq_enabled: true
  dlq_path: "./data/dlq"

# 3. Plugin Resilience: Services auto-reconnect
inputs:
  - type: kafka
    name: "events"
    config:
      brokers: ["kafka:29092"]
      topic: "logs"
      resilient: true          # Auto-reconnect
      retry_interval: 10
      max_retries: 0           # Never give up

outputs:
  - type: elasticsearch
    name: "main-index"
    config:
      addresses: ["http://elasticsearch:9200"]
      index: "logs-{yyyy.MM.dd}"
      resilient: true          # Auto-reconnect
      retry_interval: 10
      max_retries: 0           # Never give up
```

**Protection Matrix:**

| Failure Scenario | WAL | Buffering | Resilience | Result |
|-----------------|-----|-----------|------------|--------|
| Service Crash | ✅ | ✅ | - | Logs recovered on restart |
| Output Down (temp) | ✅ | ✅ | ✅ | Buffered + auto-reconnect |
| Output Down (long) | ✅ | ✅ (DLQ) | ✅ | Saved to DLQ for manual review |
| Network Issues | ✅ | ✅ | ✅ | Automatic retry + reconnect |
| Startup Race | - | ✅ | ✅ | Service starts, buffers until ready |

**Result: Zero log loss under all failure scenarios!** 🎯

## Troubleshooting

### Logs Not Being Retried

Check if buffering is enabled:
```bash
grep "output_buffer" config.yaml
grep "BUFFER:" loganalyzer.log
```

### DLQ Files Growing Too Large

1. Check output health: Is the service actually down?
2. Review `max_retries`: May need to increase for longer outages
3. Investigate root cause: Why are logs failing?

### High Memory Usage

Buffer queues may be too large:
```yaml
output_buffer:
  max_queue_size: 500  # Reduce from 1000
  flush_interval: 10s  # Flush more frequently
```

### Logs Lost After Restart

Ensure directories are persistent (Docker volumes):
```yaml
volumes:
  - buffer-data:/data/buffers
  - dlq-data:/data/dlq
```

## Performance Considerations

- **Memory**: Each output queue consumes memory proportional to `max_queue_size`
- **Disk I/O**: `flush_interval` controls how often retry queues are written to disk
- **CPU**: Minimal overhead - only during retries and health checks
- **Network**: Retry traffic is controlled by exponential backoff

### Tuning for Performance

**High Throughput**:
```yaml
output_buffer:
  max_queue_size: 5000   # Large queues
  flush_interval: 30s    # Less frequent flushes
```

**Low Latency**:
```yaml
output_buffer:
  max_queue_size: 100    # Small queues
  flush_interval: 5s     # Frequent flushes
  retry_interval: 5s     # Quick retries
```

**Memory Constrained**:
```yaml
output_buffer:
  max_queue_size: 100    # Small queues
  flush_interval: 5s     # Flush to disk quickly
```

## See Also

- [Plugin Resilience](README.md#-plugin-resilience) - Automatic reconnection
- [Log Persistence](README.md#-log-persistence) - Write-Ahead Logging
- [Configuration Guide](config.example.yaml) - Full configuration example
