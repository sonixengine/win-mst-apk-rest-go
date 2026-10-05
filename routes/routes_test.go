package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"master-panel-api/config"
	"master-panel-api/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestReportRoutesRegistration(t *testing.T) {
	config.AppConfig = &config.Config{
		JWTSecret: "test-secret-123",
	}

	app := fiber.New()
	SetupRoutes(app)

	// Buat token JWT valid
	claims := middleware.UserClaims{
		UUID:   "test-uuid",
		Email:  "test@example.com",
		Name:   "Tester",
		Master: true,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("test-secret-123"))

	// Test GET /api/v1/reports/agents
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/agents", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	// Status tidak boleh 404 Not Found!
	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("Expected route /api/v1/reports/agents to be found, but got 404 Not Found")
	}
}

func TestAgentConfigRouteRegistration(t *testing.T) {
	config.AppConfig = &config.Config{
		WorkerSecret: "test-worker-secret-456",
	}

	app := fiber.New()
	SetupRoutes(app)

	// 1. Without secret header -> must be 401 Unauthorized
	reqUnauthorized := httptest.NewRequest(http.MethodGet, "/api/v1/agent-config?agent=demo", nil)
	respUnauthorized, err := app.Test(reqUnauthorized)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if respUnauthorized.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized when missing secret, got %d", respUnauthorized.StatusCode)
	}

	// 2. With valid X-Master-Secret header -> must be 200 OK
	reqAuthorized := httptest.NewRequest(http.MethodGet, "/api/v1/agent-config?agent=demo", nil)
	reqAuthorized.Header.Set("X-Master-Secret", "test-worker-secret-456")
	respAuthorized, err := app.Test(reqAuthorized)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if respAuthorized.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK with valid secret, got %d", respAuthorized.StatusCode)
	}
}
