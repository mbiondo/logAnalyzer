# Test TLS Configuration

This script demonstrates how to test the TLS functionality with the generated certificates.

## Prerequisites

1. Generate test certificates:
   ```bash
   cd examples/scripts/certs
   ./generate-certs.ps1
   ```

2. Make sure you have the certificates in `examples/scripts/certs/`

## Test Commands

### 1. Start LogAnalyzer with TLS configuration
```bash
go run cmd/main.go -config test-config-tls.yaml
```

### 2. Test HTTPS input (in another terminal)
```bash
# Send logs via HTTPS (server will use server certificate)
curl -k -X POST https://localhost:8443/logs \
  -H "Content-Type: application/json" \
  -d '{"level": "info", "message": "Test log via HTTPS", "timestamp": "2025-10-30T15:00:00Z"}'
```

### 3. Test HTTP input (should still work)
```bash
# Send logs via HTTP (no TLS)
curl -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"level": "info", "message": "Test log via HTTP", "timestamp": "2025-10-30T15:00:00Z"}'
```

### 4. Test API endpoints
```bash
# Health check
curl http://localhost:9090/health

# Status
curl http://localhost:9090/status
```

## Unit Tests

### Run TLS Unit Tests
```bash
# Run the TLS-specific unit test
go test ./plugins/input/http -run TestHTTPInputWithTLS -v

# Run all HTTP input tests (including TLS)
go test ./plugins/input/http -v
```

The TLS unit test (`TestHTTPInputWithTLS`) verifies:
- HTTPS server starts correctly with TLS certificates
- TLS handshake works for incoming connections
- Log processing over secure HTTPS connections
- Proper log routing and metadata handling

## Certificate Details

- **CA Certificate**: `examples/scripts/certs/ca-cert.pem`
- **Server Certificate**: `examples/scripts/certs/server-cert.pem` (CN=localhost)
- **Client Certificate**: `examples/scripts/certs/client-cert.pem` (CN=client)

## Troubleshooting

1. **Connection refused**: Make sure LogAnalyzer is running
2. **SSL certificate error**: Use `-k` flag with curl to skip certificate verification for testing
3. **Port already in use**: Change ports in the configuration if needed

## Security Notes

- These are test certificates only - do not use in production
- The certificates are valid for 365 days
- Self-signed CA means browsers will show security warnings
- For production, use certificates from a trusted Certificate Authority