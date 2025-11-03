# Script to generate test certificates for LogAnalyzer TLS testing
# This creates a CA, server certificate, and client certificate

Write-Host "Generating test certificates for LogAnalyzer TLS testing..." -ForegroundColor Green

# Create CA private key
openssl genrsa -out ca-key.pem 2048

# Create CA certificate
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 365 -out ca-cert.pem -subj "/C=US/ST=Test/L=Test/O=Test/CN=TestCA"

# Create server private key
openssl genrsa -out server-key.pem 2048

# Create server certificate signing request
openssl req -new -key server-key.pem -out server.csr -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost"

# Create server certificate signed by CA
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 365 -sha256

# Create client private key
openssl genrsa -out client-key.pem 2048

# Create client certificate signing request
openssl req -new -key client-key.pem -out client.csr -subj "/C=US/ST=Test/L=Test/O=Test/CN=client"

# Create client certificate signed by CA
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem -days 365 -sha256

# Clean up CSR files
Remove-Item server.csr, client.csr -ErrorAction SilentlyContinue

Write-Host "Certificates generated successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Generated files:" -ForegroundColor Yellow
Write-Host "- ca-cert.pem (CA certificate)"
Write-Host "- ca-key.pem (CA private key)"
Write-Host "- server-cert.pem (Server certificate)"
Write-Host "- server-key.pem (Server private key)"
Write-Host "- client-cert.pem (Client certificate)"
Write-Host "- client-key.pem (Client private key)"
Write-Host ""
Write-Host "You can now test TLS functionality with these certificates." -ForegroundColor Green