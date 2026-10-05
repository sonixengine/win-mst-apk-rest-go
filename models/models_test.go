package models

import "testing"

func TestTableNames(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
	}{
		{"User Table", User{}.TableName(), "users"},
		{"AgentMaster Table", AgentMaster{}.TableName(), "agent_master"},
		{"Agent Table", Agent{}.TableName(), "agent"},
		{"AgentConnection Table", AgentConnection{}.TableName(), "agent_connection"},
		{"AgentConnectionRead Table", AgentConnectionRead{}.TableName(), "agent_connection_read"},
		{"UserGroup Table", UserGroup{}.TableName(), "user_groups"},
		{"MasterSiteConfig Table", MasterSiteConfig{}.TableName(), "master_site_configs"},
		{"PlayerUser Table", PlayerUser{}.TableName(), "users"},
		{"PlayerTransaction Table", PlayerTransaction{}.TableName(), "transactions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("%s failed: expected '%s', got '%s'", tt.name, tt.expected, tt.actual)
			}
		})
	}
}
