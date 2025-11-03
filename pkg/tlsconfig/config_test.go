package tlsconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "disabled config is valid",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "basic TLS config is valid",
			config: Config{
				Enabled:            true,
				InsecureSkipVerify: true,
			},
			wantErr: false,
		},
		{
			name: "cannot specify both ca_cert and ca_cert_data",
			config: Config{
				Enabled:    true,
				CACert:     "/path/to/ca.pem",
				CACertData: "-----BEGIN CERTIFICATE-----",
			},
			wantErr: true,
		},
		{
			name: "cannot specify both client_cert and client_cert_data",
			config: Config{
				Enabled:        true,
				ClientCert:     "/path/to/cert.pem",
				ClientCertData: "-----BEGIN CERTIFICATE-----",
			},
			wantErr: true,
		},
		{
			name: "client cert requires client key",
			config: Config{
				Enabled:    true,
				ClientCert: "/path/to/cert.pem",
			},
			wantErr: true,
		},
		{
			name: "client key requires client cert",
			config: Config{
				Enabled:   true,
				ClientKey: "/path/to/key.pem",
			},
			wantErr: true,
		},
		{
			name: "valid MTLS config",
			config: Config{
				Enabled:    true,
				ClientCert: "/path/to/cert.pem",
				ClientKey:  "/path/to/key.pem",
			},
			wantErr: false,
		},
		{
			name: "cannot specify both client_ca_cert and client_ca_cert_data",
			config: Config{
				Enabled:          true,
				ClientCACert:     "/path/to/client-ca.pem",
				ClientCACertData: "-----BEGIN CERTIFICATE-----",
			},
			wantErr: true,
		},
		{
			name: "invalid client auth",
			config: Config{
				Enabled:    true,
				ClientAuth: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid client auth",
			config: Config{
				Enabled:    true,
				ClientAuth: "require-and-verify",
			},
			wantErr: false,
		},
		{
			name: "valid min version",
			config: Config{
				Enabled:    true,
				MinVersion: "1.2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_NewTLSConfig(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		config := Config{Enabled: false}
		tlsConfig, err := config.NewTLSConfig()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tlsConfig != nil {
			t.Errorf("expected nil, got %v", tlsConfig)
		}
	})

	t.Run("basic TLS config", func(t *testing.T) {
		config := Config{
			Enabled:            true,
			InsecureSkipVerify: true,
			ServerName:         "example.com",
		}
		tlsConfig, err := config.NewTLSConfig()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tlsConfig == nil {
			t.Fatal("expected tls config, got nil")
		}
		if !tlsConfig.InsecureSkipVerify {
			t.Errorf("expected InsecureSkipVerify=true")
		}
		if tlsConfig.ServerName != "example.com" {
			t.Errorf("expected ServerName=example.com, got %s", tlsConfig.ServerName)
		}
		if tlsConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("expected MinVersion=TLS12, got %d", tlsConfig.MinVersion)
		}
	})

	t.Run("TLS version configuration", func(t *testing.T) {
		config := Config{
			Enabled:    true,
			MinVersion: "1.2",
			MaxVersion: "1.3",
		}
		tlsConfig, err := config.NewTLSConfig()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tlsConfig == nil {
			t.Fatal("expected tls config, got nil")
		}
		if tlsConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("expected MinVersion=TLS12, got %d", tlsConfig.MinVersion)
		}
		if tlsConfig.MaxVersion != tls.VersionTLS13 {
			t.Errorf("expected MaxVersion=TLS13, got %d", tlsConfig.MaxVersion)
		}
	})
}

func TestParseClientAuth(t *testing.T) {
	tests := []struct {
		clientAuth string
		want       tls.ClientAuthType
		wantErr    bool
	}{
		{"no", tls.NoClientCert, false},
		{"request", tls.RequestClientCert, false},
		{"require", tls.RequireAnyClientCert, false},
		{"verify-if-given", tls.VerifyClientCertIfGiven, false},
		{"require-and-verify", tls.RequireAndVerifyClientCert, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.clientAuth, func(t *testing.T) {
			got, err := parseClientAuth(tt.clientAuth)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseClientAuth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseClientAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_LoadCACertPool(t *testing.T) {
	// Create a valid temporary CA certificate using crypto/x509
	tmpDir := t.TempDir()
	caCertFile := filepath.Join(tmpDir, "ca.pem")

	// Generate a valid self-signed certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Test"},
			CommonName:   "TestCA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	if err := os.WriteFile(caCertFile, certPEM, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("load from file", func(t *testing.T) {
		config := Config{
			Enabled: true,
			CACert:  caCertFile,
		}
		pool, err := config.loadCACertPool()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if pool == nil {
			t.Errorf("expected cert pool, got nil")
		}
	})

	t.Run("load from data", func(t *testing.T) {
		config := Config{
			Enabled:    true,
			CACertData: string(certPEM),
		}
		pool, err := config.loadCACertPool()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if pool == nil {
			t.Errorf("expected cert pool, got nil")
		}
	})

	t.Run("no certificate provided", func(t *testing.T) {
		config := Config{
			Enabled: true,
		}
		_, err := config.loadCACertPool()
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("invalid certificate", func(t *testing.T) {
		config := Config{
			Enabled:    true,
			CACertData: "invalid certificate data",
		}
		_, err := config.loadCACertPool()
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}
