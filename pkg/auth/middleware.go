package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Middleware represents authentication middleware
type Middleware struct {
	manager      *APIKeyManager
	requireAuth  bool
	healthBypass bool
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(manager *APIKeyManager, requireAuth, healthBypass bool) *Middleware {
	return &Middleware{
		manager:      manager,
		requireAuth:  requireAuth,
		healthBypass: healthBypass,
	}
}

// Authenticate is a middleware function that validates API keys
func (m *Middleware) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bypass authentication for health endpoint if enabled
		if m.healthBypass && r.URL.Path == "/health" && r.Method == "GET" {
			next(w, r)
			return
		}

		// If authentication is not required, proceed
		if !m.requireAuth {
			next(w, r)
			return
		}

		// Extract API key from header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			m.unauthorized(w, "Missing API key in X-API-Key header")
			return
		}

		// Validate the API key
		key, err := m.manager.Validate(apiKey)
		if err != nil {
			m.unauthorized(w, fmt.Sprintf("Invalid API key: %v", err))
			return
		}

		// Check if the key has permission for this endpoint
		if !m.hasEndpointPermission(key, r.URL.Path, r.Method) {
			m.forbidden(w, "Insufficient permissions for this endpoint")
			return
		}

		// Add key information to request context for later use
		ctx := r.Context()
		ctx = ContextWithAPIKey(ctx, key)
		r = r.WithContext(ctx)

		// Proceed to next handler
		next(w, r)
	}
}

// hasEndpointPermission checks if an API key has permission for a specific endpoint
func (m *Middleware) hasEndpointPermission(key *APIKey, path, method string) bool {
	// Define endpoint permissions
	endpointPerms := map[string][]string{
		"/health":  {"health"},
		"/metrics": {"metrics", "health"}, // metrics permission includes health
		"/status":  {"admin"},             // status requires admin permission
	}

	requiredPerms, exists := endpointPerms[path]
	if !exists {
		// Unknown endpoint, deny access
		return false
	}

	// Check if key has any of the required permissions
	for _, requiredPerm := range requiredPerms {
		if key.HasPermission(requiredPerm) {
			return true
		}
	}

	return false
}

// unauthorized sends an unauthorized response
func (m *Middleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)

	_, err := fmt.Fprintf(w, `{"error":"unauthorized","message":"%s","code":401}`, message)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Error writing unauthorized response: %v\n", err)
	}
}

// forbidden sends a forbidden response
func (m *Middleware) forbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	_, err := fmt.Fprintf(w, `{"error":"forbidden","message":"%s","code":403}`, message)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Error writing forbidden response: %v\n", err)
	}
}

// WrapHandler wraps an http.Handler with authentication
func (m *Middleware) WrapHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Authenticate(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})(w, r)
	})
}

// WrapHandlerFunc wraps an http.HandlerFunc with authentication
func (m *Middleware) WrapHandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.Authenticate(handler)
}

// RequirePermission creates a middleware that requires a specific permission
func (m *Middleware) RequirePermission(permission string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// First check if authenticated
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				m.unauthorized(w, "Missing API key in X-API-Key header")
				return
			}

			key, err := m.manager.Validate(apiKey)
			if err != nil {
				m.unauthorized(w, fmt.Sprintf("Invalid API key: %v", err))
				return
			}

			// Check specific permission
			if !key.HasPermission(permission) {
				m.forbidden(w, fmt.Sprintf("Permission '%s' required", permission))
				return
			}

			// Add key to context
			ctx := r.Context()
			ctx = ContextWithAPIKey(ctx, key)
			r = r.WithContext(ctx)

			next(w, r)
		}
	}
}

// ContextAPIKeyKey is the key used to store API key in context
type contextKey string

const ContextAPIKeyKey contextKey = "api_key"

// ContextWithAPIKey adds an API key to the request context
func ContextWithAPIKey(ctx context.Context, key *APIKey) context.Context {
	return context.WithValue(ctx, ContextAPIKeyKey, key)
}

// APIKeyFromContext retrieves an API key from the request context
func APIKeyFromContext(ctx context.Context) (*APIKey, bool) {
	key, ok := ctx.Value(ContextAPIKeyKey).(*APIKey)
	return key, ok
}

// ValidateAPIKeyHeader validates an API key from the X-API-Key header
func ValidateAPIKeyHeader(r *http.Request, manager *APIKeyManager) (*APIKey, error) {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		return nil, fmt.Errorf("missing X-API-Key header")
	}

	return manager.Validate(apiKey)
}

// RequirePermissions is a helper function to check multiple permissions
func RequirePermissions(key *APIKey, permissions ...string) bool {
	for _, perm := range permissions {
		if !key.HasPermission(perm) {
			return false
		}
	}
	return true
}

// CORSHeaders adds CORS headers for API responses
func CORSHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
