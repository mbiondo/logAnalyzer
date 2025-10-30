@echo off
REM Script to generate test certificates for LogAnalyzer TLS testing
REM This creates a CA, server certificate, and client certificate

echo Generating test certificates for LogAnalyzer TLS testing...

REM Create CA private key
openssl genrsa -out ca-key.pem 2048

REM Create CA certificate
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days 365 -out ca-cert.pem -subj "/C=US/ST=Test/L=Test/O=Test/CN=TestCA"

REM Create server private key
openssl genrsa -out server-key.pem 2048

REM Create server certificate signing request
openssl req -new -key server-key.pem -out server.csr -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost"

REM Create server certificate signed by CA
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -days 365 -sha256

REM Create client private key
openssl genrsa -out client-key.pem 2048

REM Create client certificate signing request
openssl req -new -key client-key.pem -out client.csr -subj "/C=US/ST=Test/L=Test/O=Test/CN=client"

REM Create client certificate signed by CA
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out client-cert.pem -days 365 -sha256

REM Clean up CSR files
del server.csr client.csr

echo Certificates generated successfully!
echo.
echo Generated files:
echo - ca-cert.pem (CA certificate)
echo - ca-key.pem (CA private key)
echo - server-cert.pem (Server certificate)
echo - server-key.pem (Server private key)
echo - client-cert.pem (Client certificate)
echo - client-key.pem (Client private key)
echo.
echo You can now test TLS functionality with these certificates.