package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"master-panel-api/config"

	"github.com/gofiber/fiber/v2"
)

func TestRequireWorkerSecret(t *testing.T) {
	config.AppConfig = &config.Config{
		WorkerSecret: "test-secret-123",
	}

	app := fiber.New()
	app.Get("/test-gateway", RequireWorkerSecret(), func(c *fiber.Ctx) error {
		return c.SendString("access_granted")
	})

	// 1. Missing secret
	req1 := httptest.NewRequest(http.MethodGet, "/test-gateway", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", resp1.StatusCode)
	}

	// 2. Invalid secret in header
	req2 := httptest.NewRequest(http.MethodGet, "/test-gateway", nil)
	req2.Header.Set("X-Master-Secret", "wrong-secret")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", resp2.StatusCode)
	}

	// 3. Valid secret in header
	req3 := httptest.NewRequest(http.MethodGet, "/test-gateway", nil)
	req3.Header.Set("X-Master-Secret", "test-secret-123")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", resp3.StatusCode)
	}
	body3, _ := io.ReadAll(resp3.Body)
	if string(body3) != "access_granted" {
		t.Errorf("Expected body 'access_granted', got '%s'", string(body3))
	}

	// 4. Valid secret via query parameter ?secret=...
	req4 := httptest.NewRequest(http.MethodGet, "/test-gateway?secret=test-secret-123", nil)
	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", resp4.StatusCode)
	}
}
