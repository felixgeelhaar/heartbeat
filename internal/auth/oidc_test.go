package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/go-teamhealthcheck/internal/auth"
)

func TestNewOIDCValidator_MissingConfig(t *testing.T) {
	_, err := auth.NewOIDCValidator(auth.OIDCConfig{})
	if err == nil {
		t.Error("expected error for empty config")
	}

	_, err = auth.NewOIDCValidator(auth.OIDCConfig{Issuer: "https://example.com"})
	if err == nil {
		t.Error("expected error for missing client_id")
	}
}

func TestNewOIDCValidator_InvalidIssuer(t *testing.T) {
	_, err := auth.NewOIDCValidator(auth.OIDCConfig{
		Issuer:   "http://localhost:1",
		ClientID: "test-client",
	})
	if err == nil {
		t.Error("expected error for unreachable issuer")
	}
}

func newMockOIDCServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "http://" + r.Host,
			"userinfo_endpoint":      "http://" + r.Host + "/userinfo",
			"token_endpoint":         "http://" + r.Host + "/token",
			"authorization_endpoint": "http://" + r.Host + "/authorize",
		})
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"sub":   "user-123",
			"email": "alice@example.com",
			"name":  "Alice",
		})
	})

	return httptest.NewServer(mux)
}

func TestOIDCValidator_ValidToken(t *testing.T) {
	server := newMockOIDCServer()
	defer server.Close()

	validator, err := auth.NewOIDCValidator(auth.OIDCConfig{
		Issuer:   server.URL,
		ClientID: "test-client",
		Scopes:   []string{"openid", "profile"},
	})
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	claims, err := validator.ValidateToken(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("validate valid token: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("expected subject user-123, got %s", claims.Subject)
	}
	if claims.Issuer != server.URL {
		t.Errorf("expected issuer %s, got %s", server.URL, claims.Issuer)
	}
}

func TestOIDCValidator_InvalidToken(t *testing.T) {
	server := newMockOIDCServer()
	defer server.Close()

	validator, err := auth.NewOIDCValidator(auth.OIDCConfig{
		Issuer:   server.URL,
		ClientID: "test-client",
	})
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	_, err = validator.ValidateToken(context.Background(), "invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestOIDCValidator_UserinfoServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "http://" + r.Host,
			"userinfo_endpoint":      "http://" + r.Host + "/userinfo",
			"token_endpoint":         "http://" + r.Host + "/token",
			"authorization_endpoint": "http://" + r.Host + "/authorize",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	validator, err := auth.NewOIDCValidator(auth.OIDCConfig{
		Issuer:   server.URL,
		ClientID: "test-client",
	})
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	_, err = validator.ValidateToken(context.Background(), "any-token")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestOIDCValidator_DiscoveryCaching(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "http://" + r.Host,
			"userinfo_endpoint":      "http://" + r.Host + "/userinfo",
			"token_endpoint":         "http://" + r.Host + "/token",
			"authorization_endpoint": "http://" + r.Host + "/authorize",
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	validator, err := auth.NewOIDCValidator(auth.OIDCConfig{
		Issuer:   server.URL,
		ClientID: "test-client",
	})
	if err != nil {
		t.Fatalf("create validator: %v", err)
	}

	// Call twice — discovery should be cached
	validator.ValidateToken(context.Background(), "t1")
	validator.ValidateToken(context.Background(), "t2")

	if callCount > 1 {
		t.Errorf("expected discovery called once (cached), got %d", callCount)
	}
}
