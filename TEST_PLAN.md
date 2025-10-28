# Persistence & Output Buffer Testing Plan

## ✅ Unit Tests Completed

All unit tests passing with race detector:
- ✅ Persistence tests (8/8 passing)
- ✅ Output buffer tests (10/10 passing)
- ✅ No race conditions detected

## Integration Tests (Docker)

### Test 1: Basic Persistence (App Crash Recovery)
**Objective**: Verify WAL recovers logs after app crashes

**Steps**:
1. Start docker-compose
2. Generate logs with test script
3. Force kill container (docker kill loganalyzer)
4. Restart container (docker-compose up -d)
5. Check Elasticsearch for recovered logs

**Expected**:
- WAL files created in volume
- Logs recovered on restart
- No log loss

### Test 2: Output Failure with Retry
**Objective**: Verify output buffer retries when output is temporarily unavailable

**Steps**:
1. Start docker-compose
2. Generate logs (should go to Elasticsearch)
3. Stop Elasticsearch: `docker-compose stop elasticsearch`
4. Generate more logs (should be buffered)
5. Check buffer files exist: `docker exec loganalyzer ls -la /var/loganalyzer/buffers/elasticsearch`
6. Restart Elasticsearch: `docker-compose start elasticsearch`
7. Wait for retries to succeed
8. Verify all logs in Elasticsearch

**Expected**:
- Logs buffered when ES is down
- Buffer files created in volume
- Logs delivered after ES restarts
- Stats show retries occurred

### Test 3: Dead Letter Queue (DLQ)
**Objective**: Verify permanently failed logs go to DLQ

**Steps**:
1. Start docker-compose
2. Stop Elasticsearch permanently
3. Generate logs continuously
4. Wait for max retries to exhaust (3 retries × backoff)
5. Check DLQ files: `docker exec loganalyzer ls -la /var/loganalyzer/dlq`
6. Inspect DLQ content: `docker exec loganalyzer cat /var/loganalyzer/dlq/elasticsearch-dlq.jsonl`

**Expected**:
- Logs move to DLQ after max retries
- DLQ file contains failed logs
- Stats show DLQ count

### Test 4: High Load Concurrent Processing
**Objective**: Verify no race conditions under load

**Steps**:
1. Start docker-compose
2. Generate high volume of logs (1000+ logs/sec)
3. Monitor for errors or crashes
4. Check all logs delivered

**Expected**:
- No crashes or errors
- No deadlocks
- All logs processed
- Stats accurate

### Test 5: Volume Persistence
**Objective**: Verify Docker volumes persist data correctly

**Steps**:
1. Start docker-compose
2. Generate logs
3. Stop all containers: `docker-compose down` (without -v)
4. Restart: `docker-compose up -d`
5. Check data still exists in volumes

**Expected**:
- WAL files preserved
- Buffer files preserved
- DLQ files preserved

## Manual Testing Commands

### Check Logs
```powershell
# View loganalyzer logs
docker-compose logs -f loganalyzer

# Check Prometheus metrics
curl http://localhost:9090/metrics | Select-String -Pattern "loganalyzer"
```

### Inspect Volumes
```powershell
# List volume contents
docker exec loganalyzer ls -la /var/loganalyzer/wal
docker exec loganalyzer ls -la /var/loganalyzer/buffers/elasticsearch
docker exec loganalyzer ls -la /var/loganalyzer/dlq

# Read WAL files
docker exec loganalyzer cat /var/loganalyzer/wal/wal-*.jsonl

# Read buffer files
docker exec loganalyzer cat /var/loganalyzer/buffers/elasticsearch/buffer-*.jsonl

# Read DLQ files
docker exec loganalyzer cat /var/loganalyzer/dlq/elasticsearch-dlq.jsonl
```

### Check Elasticsearch
```powershell
# Count logs
curl http://localhost:9200/logs/_count

# Search logs
curl http://localhost:9200/logs/_search?pretty

# Check index health
curl http://localhost:9200/_cat/indices?v
```

### Generate Test Logs
```powershell
# Run test data script
.\examples\scripts\test-data.ps1
```

### Container Management
```powershell
# Stop specific service
docker-compose stop elasticsearch

# Force kill
docker kill loganalyzer

# Restart
docker-compose restart loganalyzer

# View stats
docker stats loganalyzer
```

## Success Criteria

✅ All unit tests pass with race detector
✅ Logs recovered after app crash
✅ Logs delivered after output recovery
✅ DLQ captures permanently failed logs
✅ No data loss under any scenario
✅ No race conditions under load
✅ Volumes persist data across restarts

## Known Limitations

1. **DLQ Processing**: DLQ logs require manual intervention
2. **Retry Limits**: After max retries, logs go to DLQ
3. **Disk Space**: Buffer and DLQ files can grow if outputs are down long-term
4. **Memory**: Retry queue is in-memory, so very large retry backlogs may use significant memory
