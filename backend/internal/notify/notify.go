// Package notify renders notification emails from broker event payloads.
package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jorgepaez/identity-hub/internal/events"
)

// Message is the safe, rendered representation of a notification email.
type Message struct {
	To      string
	Subject string
	Body    string
}

type payload struct {
	Data struct {
		Email             string `json:"email"`
		DisplayName       string `json:"displayName"`
		VerificationToken string `json:"verificationToken"`
		InvitationToken   string `json:"invitationToken"`
	} `json:"data"`
}

// Render returns the recipient, subject, and body for an event. It only renders
// allowlisted fields, so opaque credentials and event payloads never reach email
// content accidentally.
func Render(eventType, publicBaseURL string, body []byte) (Message, error) {
	var event payload
	if err := json.Unmarshal(body, &event); err != nil {
		return Message{}, fmt.Errorf("decode notification payload: %w", err)
	}
	if event.Data.Email == "" {
		return Message{}, fmt.Errorf("event %s has no recipient", eventType)
	}

	message := Message{To: event.Data.Email}
	switch eventType {
	case events.TypeUserRegistered:
		if event.Data.VerificationToken == "" {
			return Message{}, fmt.Errorf("event %s has no verification token", eventType)
		}
		verificationURL, err := verificationURL(publicBaseURL, event.Data.VerificationToken)
		if err != nil {
			return Message{}, err
		}
		message.Subject = "Verify your Identity Hub email"
		message.Body = fmt.Sprintf("Hello %s,\n\nVerify your email address: %s\n", event.Data.DisplayName, verificationURL)
	case events.TypeUserInvited:
		if event.Data.InvitationToken == "" {
			return Message{}, fmt.Errorf("event %s has no invitation token", eventType)
		}
		acceptanceURL, err := invitationAcceptanceURL(publicBaseURL, event.Data.InvitationToken)
		if err != nil {
			return Message{}, err
		}
		message.Subject = "You have been invited to Identity Hub"
		message.Body = fmt.Sprintf("Hello %s,\n\nSet your password to accept your invitation: %s\n", event.Data.DisplayName, acceptanceURL)
	case events.TypeRefreshReuseDetected:
		message.Subject = "Security alert: refresh token reuse detected"
		message.Body = "A refresh token reuse attempt was detected. Your active sessions were revoked as a precaution.\n"
	case events.TypeAccountLocked:
		message.Subject = "Security alert: account temporarily locked"
		message.Body = "Your account was temporarily locked after repeated failed sign-in attempts.\n"
	default:
		message.Subject = "Identity Hub notification"
		message.Body = fmt.Sprintf("Hello %s,\n\nYou have a new Identity Hub notification.\n", event.Data.DisplayName)
	}
	return message, nil
}

func verificationURL(publicBaseURL, token string) (string, error) {
	return accountLinkURL(publicBaseURL, "/verify-email", token)
}

// invitationAcceptanceURL builds the frontend link an invited employee uses to
// set their password (T5). The path is not fixed by specs/03-api/openapi.yaml
// (that contract only defines the backend acceptInvitation operation), so
// /invitations/accept mirrors the existing /verify-email frontend route
// convention until T12 builds the actual page.
func invitationAcceptanceURL(publicBaseURL, token string) (string, error) {
	return accountLinkURL(publicBaseURL, "/invitations/accept", token)
}

func accountLinkURL(publicBaseURL, path, token string) (string, error) {
	base, err := url.Parse(publicBaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid public base URL")
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	base.RawQuery = url.Values{"token": []string{token}}.Encode()
	return base.String(), nil
}
