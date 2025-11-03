package auth

import (
	"testing"
	"time"
)

func TestAPIKeyManager_AddKey(t *testing.T) {
	manager := NewAPIKeyManager()

	key := &APIKey{
		ID:          "test-key",
		Secret:      "secret123",
		Permissions: []string{"read", "write"},
		CreatedAt:   time.Now(),
		Name:        "Test Key",
	}

	err := manager.AddKey(key)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	retrieved, exists := manager.GetKey("test-key")
	if !exists {
		t.Fatal("Key was not found after adding")
	}

	if retrieved.ID != key.ID {
		t.Errorf("Expected ID %s, got %s", key.ID, retrieved.ID)
	}

	if retrieved.Secret != key.Secret {
		t.Errorf("Expected secret %s, got %s", key.Secret, retrieved.Secret)
	}
}

func TestAPIKeyManager_Validate(t *testing.T) {
	manager := NewAPIKeyManager()

	key := &APIKey{
		ID:          "test-key",
		Secret:      "secret123",
		Permissions: []string{"read", "write"},
		CreatedAt:   time.Now(),
		Name:        "Test Key",
	}

	err := manager.AddKey(key)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// Test valid key
	validKey, err := manager.Validate("secret123")
	if err != nil {
		t.Fatalf("Failed to validate valid key: %v", err)
	}

	if validKey.ID != "test-key" {
		t.Errorf("Expected key ID 'test-key', got '%s'", validKey.ID)
	}

	// Test invalid key
	_, err = manager.Validate("invalid-secret")
	if err == nil {
		t.Fatal("Expected error for invalid key, got nil")
	}

	// Test empty key
	_, err = manager.Validate("")
	if err == nil {
		t.Fatal("Expected error for empty key, got nil")
	}
}

func TestAPIKeyManager_ExpiredKey(t *testing.T) {
	manager := NewAPIKeyManager()

	pastTime := time.Now().Add(-1 * time.Hour)
	key := &APIKey{
		ID:          "expired-key",
		Secret:      "secret123",
		Permissions: []string{"read"},
		CreatedAt:   time.Now(),
		ExpiresAt:   &pastTime,
		Name:        "Expired Key",
	}

	err := manager.AddKey(key)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	_, err = manager.Validate("secret123")
	if err == nil {
		t.Fatal("Expected error for expired key, got nil")
	}

	if err.Error() != "API key has expired" {
		t.Errorf("Expected 'API key has expired' error, got '%s'", err.Error())
	}
}

func TestAPIKeyManager_HasPermission(t *testing.T) {
	manager := NewAPIKeyManager()

	key := &APIKey{
		ID:          "test-key",
		Secret:      "secret123",
		Permissions: []string{"read", "write"},
		CreatedAt:   time.Now(),
		Name:        "Test Key",
	}

	err := manager.AddKey(key)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// Test existing permission
	hasPerm, err := manager.HasPermission("secret123", "read")
	if err != nil {
		t.Fatalf("Error checking permission: %v", err)
	}
	if !hasPerm {
		t.Error("Expected key to have 'read' permission")
	}

	// Test non-existing permission
	hasPerm, err = manager.HasPermission("secret123", "admin")
	if err != nil {
		t.Fatalf("Error checking permission: %v", err)
	}
	if hasPerm {
		t.Error("Expected key to not have 'admin' permission")
	}

	// Test invalid key
	_, err = manager.HasPermission("invalid", "read")
	if err == nil {
		t.Fatal("Expected error for invalid key, got nil")
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	// Test non-expired key
	key := &APIKey{
		ID:        "test-key",
		CreatedAt: time.Now(),
	}

	if key.IsExpired() {
		t.Error("Expected non-expired key to not be expired")
	}

	// Test expired key
	pastTime := time.Now().Add(-1 * time.Hour)
	key.ExpiresAt = &pastTime

	if !key.IsExpired() {
		t.Error("Expected expired key to be expired")
	}
}

func TestAPIKey_HasPermission(t *testing.T) {
	key := &APIKey{
		Permissions: []string{"read", "write", "admin"},
	}

	if !key.HasPermission("read") {
		t.Error("Expected key to have 'read' permission")
	}

	if !key.HasPermission("write") {
		t.Error("Expected key to have 'write' permission")
	}

	if !key.HasPermission("admin") {
		t.Error("Expected key to have 'admin' permission")
	}

	if key.HasPermission("delete") {
		t.Error("Expected key to not have 'delete' permission")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey("Test Key", "A test API key", []string{"read", "write"}, nil)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if key.ID == "" {
		t.Error("Generated key should have an ID")
	}

	if key.Secret == "" {
		t.Error("Generated key should have a secret")
	}

	if len(key.Secret) != 64 { // 32 bytes * 2 hex chars per byte
		t.Errorf("Expected secret length 64, got %d", len(key.Secret))
	}

	if key.Name != "Test Key" {
		t.Errorf("Expected name 'Test Key', got '%s'", key.Name)
	}

	if key.Description != "A test API key" {
		t.Errorf("Expected description 'A test API key', got '%s'", key.Description)
	}

	if len(key.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(key.Permissions))
	}

	if key.ExpiresAt != nil {
		t.Error("Expected no expiration, but got one")
	}
}

func TestGenerateAPIKey_WithExpiration(t *testing.T) {
	futureTime := time.Now().Add(24 * time.Hour)

	key, err := GenerateAPIKey("Test Key", "A test API key", []string{"read"}, &futureTime)
	if err != nil {
		t.Fatalf("Failed to generate API key: %v", err)
	}

	if key.ExpiresAt == nil {
		t.Fatal("Expected expiration time to be set")
	}

	if !key.ExpiresAt.Equal(futureTime) {
		t.Errorf("Expected expiration %v, got %v", futureTime, key.ExpiresAt)
	}
}

func TestAPIKeyManager_LoadKeys(t *testing.T) {
	manager := NewAPIKeyManager()

	configKeys := []APIKeyConfig{
		{
			ID:          "key1",
			Secret:      "secret1",
			Permissions: []string{"read"},
			Name:        "Key 1",
			Description: "First key",
		},
		{
			ID:          "key2",
			Secret:      "secret2",
			Permissions: []string{"read", "write"},
			Name:        "Key 2",
			Description: "Second key",
		},
	}

	err := manager.LoadKeys(configKeys)
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	// Verify first key
	key1, exists := manager.GetKey("key1")
	if !exists {
		t.Fatal("Key1 was not loaded")
	}

	if key1.Secret != "secret1" {
		t.Errorf("Expected secret 'secret1', got '%s'", key1.Secret)
	}

	if len(key1.Permissions) != 1 || key1.Permissions[0] != "read" {
		t.Errorf("Expected permissions ['read'], got %v", key1.Permissions)
	}

	// Verify second key
	key2, exists := manager.GetKey("key2")
	if !exists {
		t.Fatal("Key2 was not loaded")
	}

	if len(key2.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(key2.Permissions))
	}
}

func TestAPIKeyManager_RemoveKey(t *testing.T) {
	manager := NewAPIKeyManager()

	key := &APIKey{
		ID:     "test-key",
		Secret: "secret123",
		Name:   "Test Key",
	}

	err := manager.AddKey(key)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// Verify key exists
	_, exists := manager.GetKey("test-key")
	if !exists {
		t.Fatal("Key should exist before removal")
	}

	// Remove key
	manager.RemoveKey("test-key")

	// Verify key no longer exists
	_, exists = manager.GetKey("test-key")
	if exists {
		t.Fatal("Key should not exist after removal")
	}
}

func TestAPIKeyManager_ListKeys(t *testing.T) {
	manager := NewAPIKeyManager()

	key1 := &APIKey{
		ID:          "key1",
		Secret:      "secret1",
		Permissions: []string{"read"},
		Name:        "Key 1",
		CreatedAt:   time.Now(),
	}

	key2 := &APIKey{
		ID:          "key2",
		Secret:      "secret2",
		Permissions: []string{"write"},
		Name:        "Key 2",
		CreatedAt:   time.Now(),
	}

	err := manager.AddKey(key1)
	if err != nil {
		t.Fatalf("Failed to add key1: %v", err)
	}
	err = manager.AddKey(key2)
	if err != nil {
		t.Fatalf("Failed to add key2: %v", err)
	}

	keys := manager.ListKeys()

	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	// Verify secrets are not included
	for _, key := range keys {
		if key.Secret != "" {
			t.Errorf("Secret should not be included in list for key %s", key.ID)
		}
	}
}
