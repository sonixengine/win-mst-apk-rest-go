package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"master-panel-api/middleware"

	"github.com/gofiber/fiber/v2"
)

func TestListAgents_NoMasterAgent(t *testing.T) {
	app := fiber.New()
	app.Get("/agents", func(c *fiber.Ctx) error {
		// Mock non-super master user claims with no MasterAgentID
		c.Locals("user", &middleware.UserClaims{
			Master:        false,
			MasterAgentID: nil,
		})
		return ListAgents(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/agents?status=active", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for empty non-master list, got %d", resp.StatusCode)
	}
}
