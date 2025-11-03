package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// APIKey represents an API key with its metadata and permissions
type APIKey struct {
	ID          string     `json:"id" yaml:"id"`
	Secret      string     `json:"-" yaml:"secret"` // Never serialize the secret
	Permissions []string   `json:"permissions" yaml:"permissions"`
	CreatedAt   time.Time  `json:"created_at" yaml:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// HasPermission checks if the API key has a specific permission
func (k *APIKey) HasPermission(permission string) bool {
	for _, p := range k.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// APIKeyManager manages API keys with thread-safe operations
type APIKeyManager struct {
	keys map[string]*APIKey // key ID -> API key
	mu   sync.RWMutex
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys: make(map[string]*APIKey),
	}
}

// AddKey adds an API key to the manager
func (m *APIKeyManager) AddKey(key *APIKey) error {
	if key.ID == "" {
		return fmt.Errorf("API key ID cannot be empty")
	}
	if key.Secret == "" {
		return fmt.Errorf("API key secret cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.keys[key.ID] = key
	return nil
}

// RemoveKey removes an API key from the manager
func (m *APIKeyManager) RemoveKey(keyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.keys, keyID)
}

// GetKey retrieves an API key by ID
func (m *APIKeyManager) GetKey(keyID string) (*APIKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	return key, exists
}

// Validate validates an API key secret and returns the key if valid
func (m *APIKeyManager) Validate(secret string) (*APIKey, error) {
	if secret == "" {
		return nil, fmt.Errorf("API key secret cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the key by secret (this is O(n), but acceptable for small number of keys)
	for _, key := range m.keys {
		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(key.Secret), []byte(secret)) == 1 {
			// Check if key is expired
			if key.IsExpired() {
				return nil, fmt.Errorf("API key has expired")
			}
			return key, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

// HasPermission checks if an API key has a specific permission
func (m *APIKeyManager) HasPermission(secret, permission string) (bool, error) {
	key, err := m.Validate(secret)
	if err != nil {
		return false, err
	}

	return key.HasPermission(permission), nil
}

// ListKeys returns a copy of all API keys (without secrets)
func (m *APIKeyManager) ListKeys() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		// Return a copy without the secret
		keyCopy := &APIKey{
			ID:          key.ID,
			Permissions: make([]string, len(key.Permissions)),
			CreatedAt:   key.CreatedAt,
			ExpiresAt:   key.ExpiresAt,
			Name:        key.Name,
			Description: key.Description,
		}
		copy(keyCopy.Permissions, key.Permissions)
		keys = append(keys, keyCopy)
	}

	return keys
}

// GenerateAPIKey generates a new API key with a random secret
func GenerateAPIKey(name, description string, permissions []string, expiresAt *time.Time) (*APIKey, error) {
	// Generate a random 32-byte secret
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)

	// Generate a random ID (8 bytes, hex encoded)
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	key := &APIKey{
		ID:          id,
		Secret:      secret,
		Permissions: make([]string, len(permissions)),
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
		Name:        name,
		Description: description,
	}
	copy(key.Permissions, permissions)

	return key, nil
}

// LoadKeys loads API keys from a configuration map
func (m *APIKeyManager) LoadKeys(configKeys []APIKeyConfig) error {
	for _, config := range configKeys {
		// Parse permissions
		permissions := make([]string, len(config.Permissions))
		copy(permissions, config.Permissions)

		// Parse expiration time if provided
		var expiresAt *time.Time
		if config.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, config.ExpiresAt); err == nil {
				expiresAt = &t
			} else {
				return fmt.Errorf("invalid expiration time format for key %s: %w", config.ID, err)
			}
		}

		key := &APIKey{
			ID:          config.ID,
			Secret:      config.Secret,
			Permissions: permissions,
			CreatedAt:   time.Now(), // Default to now if not specified
			ExpiresAt:   expiresAt,
			Name:        config.Name,
			Description: config.Description,
		}

		if err := m.AddKey(key); err != nil {
			return fmt.Errorf("failed to add key %s: %w", config.ID, err)
		}
	}

	return nil
}

// APIKeyConfig represents the configuration for an API key
type APIKeyConfig struct {
	ID          string   `json:"id" yaml:"id"`
	Secret      string   `json:"secret" yaml:"secret"`
	Permissions []string `json:"permissions" yaml:"permissions"`
	ExpiresAt   string   `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}
