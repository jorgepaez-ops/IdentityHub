package notify

import (
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/events"
)

func TestRF012_PlantillaDeRegistroIncluyeElEnlaceDeVerificacion(t *testing.T) {
	message, err := Render(events.TypeUserRegistered, "https://id.example", []byte(`{
		"data": {
			"email": "ana@example.com",
			"displayName": "Ana",
			"verificationToken": "token-con-espacio"
		}
	}`))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if message.To != "ana@example.com" {
		t.Errorf("To = %q, want ana@example.com", message.To)
	}
	if !strings.Contains(message.Body, "https://id.example/verify-email?token=token-con-espacio") {
		t.Errorf("registration message does not contain verification URL: %q", message.Body)
	}
}

func TestRF001_PlantillaDeInvitacionIncluyeElEnlaceDeAceptacion(t *testing.T) {
	message, err := Render(events.TypeUserInvited, "https://id.example", []byte(`{
		"data": {
			"email": "ana@example.com",
			"displayName": "Ana",
			"invitationToken": "token-con-espacio"
		}
	}`))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if message.To != "ana@example.com" {
		t.Errorf("To = %q, want ana@example.com", message.To)
	}
	if !strings.Contains(message.Body, "https://id.example/invitations/accept?token=token-con-espacio") {
		t.Errorf("invitation message does not contain acceptance URL: %q", message.Body)
	}
}

func TestRF012_LosAvisosDeSeguridadNoIncluyenSecretos(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		payload   string
		secrets   []string
	}{
		{
			name:      "refresh token reuse",
			eventType: events.TypeRefreshReuseDetected,
			payload:   `{"data":{"email":"ana@example.com","familyId":"family-secret","refreshToken":"refresh-secret","revokedCount":2}}`,
			secrets:   []string{"family-secret", "refresh-secret"},
		},
		{
			name:      "account locked",
			eventType: events.TypeAccountLocked,
			payload:   `{"data":{"email":"ana@example.com","password":"password-secret","failedAttempts":5}}`,
			secrets:   []string{"password-secret"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message, err := Render(tc.eventType, "https://id.example", []byte(tc.payload))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(message.Subject, "Security") {
				t.Errorf("Subject = %q, want a security notice", message.Subject)
			}
			for _, secret := range tc.secrets {
				if strings.Contains(message.Body, secret) {
					t.Errorf("security message leaked %q: %q", secret, message.Body)
				}
			}
		})
	}
}

// events.PasswordResetRequested never carries a display name (see
// events.go), so this fixture omits it: the neutral greeting is the only
// greeting this event can ever produce, not a fallback for a missing name.
func TestRF015_PlantillaDeRestablecimientoIncluyeElEnlaceYSaludoNeutral(t *testing.T) {
	message, err := Render(events.TypePasswordResetRequested, "https://id.example", []byte(`{
		"data": {
			"email": "ada@example.com",
			"resetToken": "reset-token"
		}
	}`))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if message.To != "ada@example.com" {
		t.Errorf("To = %q", message.To)
	}
	if !strings.Contains(message.Body, "https://id.example/password-reset?token=reset-token") {
		t.Fatalf("reset message does not contain reset URL: %q", message.Body)
	}
	if !strings.Contains(message.Body, "Hello there,") {
		t.Fatalf("reset message has no neutral greeting: %q", message.Body)
	}
}
