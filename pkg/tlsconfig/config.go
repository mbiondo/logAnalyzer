package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
)

// Config represents TLS configuration options
type Config struct {
	// Enable TLS
	Enabled bool `yaml:"enabled,omitempty"`

	// Server Certificate Validation
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"` // Skip certificate verification (DANGER: only for development!)
	CACert             string `yaml:"ca_cert,omitempty"`              // Path to CA certificate file
	CACertData         string `yaml:"ca_cert_data,omitempty"`         // CA certificate data (base64 or PEM)

	// Client Certificate (MTLS)
	ClientCert     string `yaml:"client_cert,omitempty"`      // Path to client certificate file
	ClientCertData string `yaml:"client_cert_data,omitempty"` // Client certificate data (base64 or PEM)
	ClientKey      string `yaml:"client_key,omitempty"`       // Path to client key file
	ClientKeyData  string `yaml:"client_key_data,omitempty"`  // Client key data (base64 or PEM)

	// TLS Version
	MinVersion string `yaml:"min_version,omitempty"` // Minimum TLS version: "1.0", "1.1", "1.2", "1.3" (default: "1.2")
	MaxVersion string `yaml:"max_version,omitempty"` // Maximum TLS version: "1.0", "1.1", "1.2", "1.3"

	// Server Name (SNI)
	ServerName string `yaml:"server_name,omitempty"` // Server name for SNI

	// Server-side Client Certificate Verification (for MTLS servers)
	ClientCACert     string `yaml:"client_ca_cert,omitempty"`      // Path to CA certificate for client verification
	ClientCACertData string `yaml:"client_ca_cert_data,omitempty"` // CA certificate data for client verification
	ClientAuth       string `yaml:"client_auth,omitempty"`         // Client auth mode: "no", "request", "require", "verify-if-given", "require-and-verify"
}

// NewTLSConfig creates a *tls.Config from the TLS configuration
func (c *Config) NewTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	// Security warning for InsecureSkipVerify
	if c.InsecureSkipVerify {
		log.Printf("WARNING: TLS InsecureSkipVerify is enabled. This disables certificate verification and should only be used in development environments!")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.InsecureSkipVerify, // #nosec G402 - intentionally configurable for development
		ServerName:         c.ServerName,
	}

	// Set TLS version constraints
	if c.MinVersion != "" {
		version, err := parseTLSVersion(c.MinVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid min_version: %w", err)
		}
		tlsConfig.MinVersion = version
	} else {
		// Default to TLS 1.2 minimum
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	if c.MaxVersion != "" {
		version, err := parseTLSVersion(c.MaxVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid max_version: %w", err)
		}
		tlsConfig.MaxVersion = version
	}

	// Load CA certificate for server verification
	if c.CACert != "" || c.CACertData != "" {
		certPool, err := c.loadCACertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		tlsConfig.RootCAs = certPool
	}

	// Load client certificate for mutual TLS (MTLS)
	if c.ClientCert != "" || c.ClientCertData != "" {
		cert, err := c.loadClientCertificate()
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load client CA certificate for server-side client verification (MTLS)
	if c.ClientCACert != "" || c.ClientCACertData != "" {
		clientCertPool, err := c.loadClientCACertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to load client CA certificate: %w", err)
		}
		tlsConfig.ClientCAs = clientCertPool
	}

	// Set client authentication mode
	if c.ClientAuth != "" {
		clientAuth, err := parseClientAuth(c.ClientAuth)
		if err != nil {
			return nil, fmt.Errorf("invalid client_auth: %w", err)
		}
		tlsConfig.ClientAuth = clientAuth
	}

	return tlsConfig, nil
}

// loadCACertPool loads the CA certificate pool from file or data
func (c *Config) loadCACertPool() (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	var caCertData []byte
	var err error

	// Load from file or use provided data
	if c.CACert != "" {
		caCertData, err = os.ReadFile(c.CACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert file: %w", err)
		}
	} else if c.CACertData != "" {
		caCertData = []byte(c.CACertData)
	} else {
		return nil, fmt.Errorf("no CA certificate provided")
	}

	// Add certificate to pool
	if !certPool.AppendCertsFromPEM(caCertData) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return certPool, nil
}

// loadClientCACertPool loads the client CA certificate pool for server-side client verification
func (c *Config) loadClientCACertPool() (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	var caCertData []byte
	var err error

	// Load from file or use provided data
	if c.ClientCACert != "" {
		caCertData, err = os.ReadFile(c.ClientCACert)
		if err != nil {
			return nil, fmt.Errorf("failed to read client CA cert file: %w", err)
		}
	} else if c.ClientCACertData != "" {
		caCertData = []byte(c.ClientCACertData)
	} else {
		return nil, fmt.Errorf("no client CA certificate provided")
	}

	// Add certificate to pool
	if !certPool.AppendCertsFromPEM(caCertData) {
		return nil, fmt.Errorf("failed to parse client CA certificate")
	}

	return certPool, nil
}

// loadClientCertificate loads the client certificate and key for MTLS
func (c *Config) loadClientCertificate() (tls.Certificate, error) {
	var certData, keyData []byte
	var err error

	// Load certificate
	if c.ClientCert != "" {
		certData, err = os.ReadFile(c.ClientCert)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to read client cert file: %w", err)
		}
	} else if c.ClientCertData != "" {
		certData = []byte(c.ClientCertData)
	} else {
		return tls.Certificate{}, fmt.Errorf("no client certificate provided")
	}

	// Load key
	if c.ClientKey != "" {
		keyData, err = os.ReadFile(c.ClientKey)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to read client key file: %w", err)
		}
	} else if c.ClientKeyData != "" {
		keyData = []byte(c.ClientKeyData)
	} else {
		return tls.Certificate{}, fmt.Errorf("no client key provided")
	}

	// Load certificate and key pair
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load key pair: %w", err)
	}

	return cert, nil
}

// parseTLSVersion parses a TLS version string to uint16
func parseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unknown TLS version: %s (supported: 1.0, 1.1, 1.2, 1.3)", version)
	}
}

// parseClientAuth parses a client auth string to tls.ClientAuthType
func parseClientAuth(clientAuth string) (tls.ClientAuthType, error) {
	switch clientAuth {
	case "no":
		return tls.NoClientCert, nil
	case "request":
		return tls.RequestClientCert, nil
	case "require":
		return tls.RequireAnyClientCert, nil
	case "verify-if-given":
		return tls.VerifyClientCertIfGiven, nil
	case "require-and-verify":
		return tls.RequireAndVerifyClientCert, nil
	default:
		return 0, fmt.Errorf("unknown client_auth: %s (supported: no, request, require, verify-if-given, require-and-verify)", clientAuth)
	}
}

// Validate validates the TLS configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Security validation for InsecureSkipVerify
	if c.InsecureSkipVerify {
		log.Printf("SECURITY WARNING: TLS InsecureSkipVerify is enabled. This disables certificate verification and should NEVER be used in production!")
	}

	// Validate CA certificate
	if c.CACert != "" && c.CACertData != "" {
		return fmt.Errorf("cannot specify both ca_cert and ca_cert_data")
	}

	// Validate client certificate
	if c.ClientCert != "" && c.ClientCertData != "" {
		return fmt.Errorf("cannot specify both client_cert and client_cert_data")
	}

	// Validate client key
	if c.ClientKey != "" && c.ClientKeyData != "" {
		return fmt.Errorf("cannot specify both client_key and client_key_data")
	}

	// Both certificate and key are required for MTLS
	hasCert := c.ClientCert != "" || c.ClientCertData != ""
	hasKey := c.ClientKey != "" || c.ClientKeyData != ""

	if hasCert != hasKey {
		return fmt.Errorf("both client certificate and key must be provided for MTLS")
	}

	// Validate TLS versions
	if c.MinVersion != "" {
		if _, err := parseTLSVersion(c.MinVersion); err != nil {
			return err
		}
	}

	if c.MaxVersion != "" {
		if _, err := parseTLSVersion(c.MaxVersion); err != nil {
			return err
		}
	}

	// Validate client CA certificate
	if c.ClientCACert != "" && c.ClientCACertData != "" {
		return fmt.Errorf("cannot specify both client_ca_cert and client_ca_cert_data")
	}

	// Validate client auth
	if c.ClientAuth != "" {
		if _, err := parseClientAuth(c.ClientAuth); err != nil {
			return err
		}
	}

	return nil
}
