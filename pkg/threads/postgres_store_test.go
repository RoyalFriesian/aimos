package threads

import "testing"

func TestPersistedMessageRole(t *testing.T) {
	tests := []struct {
		name        string
		authorRole  string
		messageType string
		want        Role
	}{
		{name: "client role maps to user", authorRole: "client", messageType: "client_message", want: RoleUser},
		{name: "ceo role maps to assistant", authorRole: "ceo", messageType: "ceo_message", want: RoleAssistant},
		{name: "system role maps to system", authorRole: "system", messageType: "timer_triggered", want: RoleSystem},
		{name: "explicit user role preserved", authorRole: string(RoleUser), messageType: "client_message", want: RoleUser},
		{name: "fallback by message type", authorRole: "unknown", messageType: "client_action_request", want: RoleUser},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := persistedMessageRole(test.authorRole, test.messageType); got != test.want {
				t.Fatalf("persistedMessageRole(%q, %q) = %q, want %q", test.authorRole, test.messageType, got, test.want)
			}
		})
	}
}
