# Test Certificates for LogAnalyzer TLS

This directory contains scripts to generate test certificates for TLS/MTLS testing with LogAnalyzer.

## Generated Certificates

Files created by `examples/scripts/certs/generate-certs.sh` or `examples/scripts/certs/generate-certs.ps1`:

- `ca-cert.pem` - Certificate Authority certificate
- `ca-key.pem` - Certificate Authority private key (keep private; used only for signing example certs)
- `server-cert.pem` - Server certificate (for HTTPS input)
- `server-key.pem` - Server private key
- `client-cert.pem` - Client certificate (for MTLS)
- `client-key.pem` - Client private key

## Usage

### Generate Certificates

Run one of these commands from this directory:
- On Linux/macOS/git-bash: `./scripts/certs/generate-certs.sh`
- On Windows PowerShell: `./scripts/certs/generate-certs.ps1`

The generated files will be placed in `examples/certs/` and the example configuration `examples/loganalyzer-tls.yaml` references paths like `./examples/certs/ca-cert.pem` and `./examples/certs/server-cert.pem`.

**Security note:** These certificates are for local testing only. Do NOT use them in production.

### Test TLS Configuration

Use these certificates in your LogAnalyzer configuration:

```yaml
inputs:
  - type: http
    name: "http-tls"
    config:
      port: 8443
      tls:
        enabled: true
        cert_file: "examples/scripts/certs/server-cert.pem"
        key_file: "examples/scripts/certs/server-key.pem"
        ca_cert_file: "examples/scripts/certs/ca-cert.pem"

outputs:
  elasticsearch:
    addresses: ["https://localhost:9200"]
    tls:
      enabled: true
      ca_cert_file: "examples/scripts/certs/ca-cert.pem"
      client_cert_file: "examples/scripts/certs/client-cert.pem"
      client_key_file: "examples/scripts/certs/client-key.pem"
```

## Security Note

These are test certificates with weak security settings. Do not use them in production!

- Valid for 365 days only
- Use weak 2048-bit RSA keys
- Self-signed CA
- No proper certificate validation

For production, use certificates from a trusted Certificate Authority.