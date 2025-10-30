# Test Certificates for LogAnalyzer TLS<!-- README for test certs created under examples/certs -->

# Test certificates for LogAnalyzer examples

This directory contains scripts to generate test certificates for TLS/MTLS testing with LogAnalyzer.

This directory is intended to contain test TLS/MTLS certificates used by the examples in `examples/`.

## Generated Certificates

Files created by `examples/scripts/certs/generate-certs.sh` or `examples/scripts/certs/generate-certs.ps1`:

After running the generation script, you'll have:

- `ca.pem` - Certificate Authority public certificate

- `ca-cert.pem` - Certificate Authority certificate- `ca.key` - Certificate Authority private key (keep private; used only for signing example certs)

- `ca-key.pem` - Certificate Authority private key- `server.pem` - Server certificate (signed by the CA)

- `server-cert.pem` - Server certificate (for HTTPS input)- `server.key` - Server private key

- `server-key.pem` - Server private key- `server-cert.pem` - Convenience file combining `server.pem` + `server.key`

- `client-cert.pem` - Client certificate (for MTLS)- `client.pem` - Client certificate (for MTLS)

- `client-key.pem` - Client private key- `client.key` - Client private key



## UsageUsage:



### Generate Certificates1. From the `examples` directory run the appropriate script:



Run one of these commands from this directory:   - On Linux/macOS/git-bash: `./scripts/certs/generate-certs.sh`

   - On Windows PowerShell: `./scripts/certs/generate-certs.ps1`

**PowerShell (recommended):**

```powershell2. The generated files will be placed in `examples/certs/` and the example configuration `examples/loganalyzer-tls.yaml`

.\generate-certs.ps1   references paths like `./examples/certs/ca.pem` and `./examples/certs/server.pem`.

```

Security note:

**Batch file:**These certificates are for local testing only. Do NOT use them in production.

```cmd
generate-certs.bat
```

### Test TLS Configuration

Use these certificates in your LogAnalyzer configuration:

```yaml
inputs:
  http:
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