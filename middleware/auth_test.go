package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"master-panel-api/config"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestProtectedMiddleware(t *testing.T) {
	jwtSecret := "unit-test-jwt-secret-key"
	config.AppConfig = &config.Config{
		JWTSecret: jwtSecret,
	}

	app := fiber.New()
	app.Get("/test-protected", Protected(), func(c *fiber.Ctx) error {
		user := c.Locals("user").(*UserClaims)
		return c.SendString(fmt.Sprintf("welcome:%s", user.Email))
	})

	// 1. Missing header
	req1 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if resp1.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for missing token, got %d", resp1.StatusCode)
	}

	// 2. Malformed Authorization header
	req2 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req2.Header.Set("Authorization", "Basic 12345")
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for invalid format, got %d", resp2.StatusCode)
	}

	// 3. Invalid / forged token
	req3 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req3.Header.Set("Authorization", "Bearer invalid.token.payload")
	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if resp3.StatusCode != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for bad token, got %d", resp3.StatusCode)
	}

	// 4. Valid token
	claims := UserClaims{
		UUID:   "test-uuid-999",
		Email:  "admin@example.com",
		Name:   "Admin Test",
		Master: true,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(jwtSecret))

	req4 := httptest.NewRequest(http.MethodGet, "/test-protected", nil)
	req4.Header.Set("Authorization", "Bearer "+tokenStr)
	resp4, err := app.Test(req4)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for valid token, got %d", resp4.StatusCode)
	}
	body4, _ := io.ReadAll(resp4.Body)
	if string(body4) != "welcome:admin@example.com" {
		t.Errorf("Expected body 'welcome:admin@example.com', got '%s'", string(body4))
	}
}

func TestUserClaims_IsSuperMaster(t *testing.T) {
	// Case 1: Nil claims
	var nilClaims *UserClaims
	if nilClaims.IsSuperMaster() {
		t.Error("Expected nil claims to return false")
	}

	// Case 2: Master = true
	c1 := &UserClaims{Master: true}
	if !c1.IsSuperMaster() {
		t.Error("Expected IsSuperMaster to return true when Master=true")
	}

	// Case 3: MasterAgentID = 1
	masterID1 := uint(1)
	c2 := &UserClaims{MasterAgentID: &masterID1, Master: false}
	if !c2.IsSuperMaster() {
		t.Error("Expected IsSuperMaster to return true when MasterAgentID=1")
	}

	// Case 4: MasterAgentID != 1 and Master = false
	masterID2 := uint(2)
	c3 := &UserClaims{MasterAgentID: &masterID2, Master: false}
	if c3.IsSuperMaster() {
		t.Error("Expected IsSuperMaster to return false when MasterAgentID!=1 and Master=false")
	}
}

