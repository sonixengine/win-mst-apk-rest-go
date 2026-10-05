package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"master-panel-api/middleware"

	"github.com/gofiber/fiber/v2"
)

func TestGetAgentWinloseReport_AccessForbidden(t *testing.T) {
	app := fiber.New()
	app.Get("/reports/winlose/agent/:uuid", func(c *fiber.Ctx) error {
		// Mock non-super master user claims
		masterID := uint(99)
		c.Locals("user", &middleware.UserClaims{
			Master:        false,
			MasterAgentID: &masterID,
		})
		return GetAgentWinloseReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/reports/winlose/agent/non-existent-uuid", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	// Should reject or return 404/403/503 since agent does not exist in DB or DB uninitialized
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected 404, 403, 500, or 503 status code, got %d", resp.StatusCode)
	}
}

func TestAgentWinloseReportStructSerialization(t *testing.T) {
	report := AgentWinloseReport{
		AgentUUID:        "test-uuid-1",
		AgentCode:        "win",
		AgentName:        "Win Gaming",
		StartDate:        "2026-09-01",
		EndDate:          "2026-09-30",
		TotalDeposit:     1000000,
		DepositCount:     10,
		TotalWithdraw:    600000,
		WithdrawCount:    4,
		FinancialWinlose: 400000,
		TotalTurnover:    5000000,
		TotalWin:         4800000,
		GameWinlose:      200000,
		TotalBetround:    150,
		CategoryBreakdown: []WinloseCategoryBreakdown{
			{
				GameCategory:  "slots",
				TotalBetround: 100,
				Turnover:      3000000,
				Win:           2900000,
				Profit:        100000,
			},
		},
		Status: "ok",
	}

	bytes, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to serialize report: %v", err)
	}

	var parsed AgentWinloseReport
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("Failed to deserialize report: %v", err)
	}

	if parsed.FinancialWinlose != 400000 {
		t.Errorf("Expected FinancialWinlose 400000, got %v", parsed.FinancialWinlose)
	}
	if parsed.GameWinlose != 200000 {
		t.Errorf("Expected GameWinlose 200000, got %v", parsed.GameWinlose)
	}
	if len(parsed.CategoryBreakdown) != 1 || parsed.CategoryBreakdown[0].GameCategory != "slots" {
		t.Errorf("CategoryBreakdown serialization mismatch")
	}
}

func TestGetAgentActivePlayersReport_Access(t *testing.T) {
	app := fiber.New()
	app.Get("/reports/winlose/agent/:uuid/players", func(c *fiber.Ctx) error {
		masterID := uint(99)
		c.Locals("user", &middleware.UserClaims{
			Master:        false,
			MasterAgentID: &masterID,
		})
		return GetAgentActivePlayersReport(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/reports/winlose/agent/non-existent-uuid/players", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected 404, 403, 500, or 503 status code, got %d", resp.StatusCode)
	}
}

func TestActivePlayersResponseSerialization(t *testing.T) {
	resp := ActivePlayersResponse{
		AgentUUID:  "agent-123",
		AgentCode:  "win",
		AgentName:  "Win Agent",
		StartDate:  "2026-09-01",
		EndDate:    "2026-09-30",
		Page:       1,
		Limit:      15,
		TotalItems: 1,
		TotalPages: 1,
		Players: []ActivePlayerItem{
			{
				UserID:        1001,
				Username:      "playerone",
				ExtPlayer:     "EXT1001",
				Saldo:         250000,
				TotalBetround: 50,
				Turnover:      500000,
				Win:           450000,
				Profit:        50000,
			},
		},
		Cached: true,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to serialize ActivePlayersResponse: %v", err)
	}

	var parsed ActivePlayersResponse
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("Failed to deserialize ActivePlayersResponse: %v", err)
	}

	if parsed.TotalItems != 1 || len(parsed.Players) != 1 {
		t.Fatalf("Expected 1 player, got %d", len(parsed.Players))
	}
	if parsed.Players[0].Username != "playerone" {
		t.Errorf("Expected username playerone, got %s", parsed.Players[0].Username)
	}
	if parsed.Players[0].Profit != 50000 {
		t.Errorf("Expected profit 50000, got %v", parsed.Players[0].Profit)
	}
}
