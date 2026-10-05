package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestGetWorkerAgentConfig_MissingAgent(t *testing.T) {
	app := fiber.New()
	app.Get("/config", GetWorkerAgentConfig)

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestGetWorkerAgentConfig_DefaultFallback(t *testing.T) {
	app := fiber.New()
	app.Get("/config", GetWorkerAgentConfig)

	req := httptest.NewRequest(http.MethodGet, "/config?agent=testagent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if data["agent_code"] != "testagent" {
		t.Errorf("Expected agent_code 'testagent', got %v", data["agent_code"])
	}
	if data["app_name"] != "Win Gaming" {
		t.Errorf("Expected fallback app_name 'Win Gaming', got %v", data["app_name"])
	}
}
