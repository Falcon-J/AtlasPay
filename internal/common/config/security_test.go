package config

import (
	"strings"
	"testing"
)

func TestSecurityValidation(t *testing.T) {
	for _, env := range []string{"", "prod", "unknown"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("JWT_ACCESS_SECRET", "")
			t.Setenv("JWT_REFRESH_SECRET", "")
			if err := Load().Validate("api-gateway"); err == nil {
				t.Fatal("unsafe JWT configuration accepted")
			}
		})
	}
	t.Setenv("APP_ENV", "dev")
	if err := Load().Validate("api-gateway"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "prod")
	t.Setenv("JWT_ACCESS_SECRET", strings.Repeat("a", 32))
	t.Setenv("JWT_REFRESH_SECRET", strings.Repeat("b", 32))
	for _, value := range []string{"short", strings.Repeat("b", 32), "atlaspay-access-secret-change-in-prod", "atlaspay-access-secret-dev-only"} {
		t.Setenv("JWT_ACCESS_SECRET", value)
		if err := Load().Validate("api-gateway"); err == nil {
			t.Fatalf("accepted unsafe secret %q", value)
		}
	}
	t.Setenv("JWT_ACCESS_SECRET", strings.Repeat("a", 32))
	t.Setenv("PAYMENT_SERVICE_URL", "http://payment:8081")
	t.Setenv("PAYMENT_SERVICE_TOKEN", "")
	if err := Load().Validate("api-gateway"); err == nil {
		t.Fatal("accepted missing outbound token")
	}
	t.Setenv("PAYMENT_SERVICE_TOKEN", "configured-token")
	if err := Load().Validate("api-gateway"); err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"inventory", "order", "payment"} {
		t.Setenv(strings.ToUpper(service)+"_SERVICE_TOKEN", "")
		if err := Load().Validate(service + "-service"); err == nil {
			t.Fatal("accepted missing listener token")
		}
	}
}

func TestCORSOriginValidation(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("INVENTORY_SERVICE_TOKEN", "configured-token")
	for _, origin := range []string{"*", "null", "ftp://example.com", "https://example.com/path", "https://user@example.com", "https://example.com?x=y", "https://example.com#x", "https://*.example.com"} {
		t.Setenv("CORS_ALLOWED_ORIGINS", origin)
		if err := Load().Validate("inventory-service"); err == nil {
			t.Fatalf("accepted %q", origin)
		}
	}
	for _, origin := range []string{"", "https://example.com, http://localhost:3000"} {
		t.Setenv("CORS_ALLOWED_ORIGINS", origin)
		if err := Load().Validate("inventory-service"); err != nil {
			t.Fatal(err)
		}
	}
}
