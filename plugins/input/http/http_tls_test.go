package httpinput

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mbiondo/logAnalyzer/core"
	"github.com/mbiondo/logAnalyzer/pkg/tlsconfig"
)

// Helper functions for generating test certificates
func generateTestCACert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	return cert, priv
}

func generateTestServerCert(t *testing.T, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Test Server"},
			CommonName:   "localhost",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	return cert, priv
}

func writeCertToFile(t *testing.T, filename string, cert *x509.Certificate) {
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Failed to close file %s: %v", filename, closeErr)
		}
	}()

	err = pem.Encode(file, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err != nil {
		t.Fatal(err)
	}
}

func writeKeyToFile(t *testing.T, filename string, key *rsa.PrivateKey) {
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Failed to close file %s: %v", filename, closeErr)
		}
	}()

	err = pem.Encode(file, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPInputWithTLS(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate test certificates programmatically
	caCert, caKey := generateTestCACert(t)
	serverCert, serverKey := generateTestServerCert(t, caCert, caKey)

	// Write certificates to temporary files
	caCertFile := filepath.Join(tmpDir, "ca-cert.pem")
	serverCertFile := filepath.Join(tmpDir, "server-cert.pem")
	serverKeyFile := filepath.Join(tmpDir, "server-key.pem")

	writeCertToFile(t, caCertFile, caCert)
	writeCertToFile(t, serverCertFile, serverCert)
	writeKeyToFile(t, serverKeyFile, serverKey)

	// Create HTTP input with TLS configuration
	config := Config{
		Port:     "8443", // Use fixed port for testing
		CertFile: serverCertFile,
		KeyFile:  serverKeyFile,
		TLS: tlsconfig.Config{
			Enabled: true,
			CACert:  caCertFile,
		},
	}

	input := NewHTTPInputWithConfig(config)
	input.SetName("test-tls-input")

	// Create a channel to receive logs
	logCh := make(chan *core.Log, 10)
	input.SetLogChannel(logCh)

	// Start the input
	err := input.Start()
	if err != nil {
		t.Fatalf("Failed to start HTTP input with TLS: %v", err)
	}
	defer func() {
		if stopErr := input.Stop(); stopErr != nil {
			t.Errorf("Failed to stop input: %v", stopErr)
		}
	}()

	// Wait for the server to start and bind to a port
	time.Sleep(500 * time.Millisecond)

	// Get the actual port the server is listening on
	server := input.server
	if server == nil {
		t.Fatal("Server not initialized")
	}

	// Try to get the listener to extract the actual port
	if server.TLSConfig == nil {
		t.Fatal("TLS config not set on server")
	}

	// For testing, we'll use a fixed port since getting the actual bound port is complex
	// In a real scenario, the port would be configured
	testPort := "8443"
	testURL := fmt.Sprintf("https://localhost:%s/logs", testPort)

	// Make HTTPS request to the server
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip verification for self-signed cert
			},
		},
	}
	testBody := `{"level": "info", "message": "Test TLS log", "timestamp": "2025-10-30T15:00:00Z"}`

	resp, err := client.Post(testURL, "application/json", strings.NewReader(testBody))
	if err != nil {
		t.Fatalf("Failed to make HTTPS request: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check that the log was received
	select {
	case log := <-logCh:
		if log.Level != "info" {
			t.Errorf("Expected log level 'info', got '%s'", log.Level)
		}
		// The message should contain the key elements, allowing for JSON formatting differences
		if !strings.Contains(log.Message, `"level":"info"`) ||
			!strings.Contains(log.Message, `"message":"Test TLS log"`) ||
			!strings.Contains(log.Message, `"timestamp":"2025-10-30T15:00:00Z"`) {
			t.Errorf("Expected log message to contain TLS test data, got '%s'", log.Message)
		}
		if log.Source != "test-tls-input" {
			t.Errorf("Expected source 'test-tls-input', got '%s'", log.Source)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for log to be processed")
	}
}
