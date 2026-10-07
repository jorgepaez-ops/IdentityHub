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

type payloadData struct {
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	VerificationToken string `json:"verificationToken"`
	InvitationToken   string `json:"invitationToken"`
	ResetToken        string `json:"resetToken"`
	Code              string `json:"code"`
	Unlocked          bool   `json:"unlocked"`
}

type payload struct {
	Data payloadData `json:"data"`
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
	if strings.ContainsAny(event.Data.Email, "\r\n") {
		return Message{}, fmt.Errorf("event %s has an invalid recipient", eventType)
	}

	message := Message{To: event.Data.Email}
	var err error
	switch eventType {
	case events.TypeUserRegistered:
		message.Subject, message.Body, err = renderUserRegistered(eventType, publicBaseURL, event.Data)
	case events.TypeUserInvited:
		message.Subject, message.Body, err = renderUserInvited(eventType, publicBaseURL, event.Data)
	case events.TypePasswordResetRequested:
		message.Subject, message.Body, err = renderPasswordResetRequested(eventType, publicBaseURL, event.Data)
	case events.TypePasswordResetCompleted:
		message.Subject, message.Body = renderPasswordResetCompleted(event.Data)
	case events.TypeMfaChallengeIssued:
		message.Subject, message.Body, err = renderMfaChallengeIssued(eventType, event.Data)
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
	if err != nil {
		return Message{}, err
	}
	return message, nil
}

func renderUserRegistered(eventType, publicBaseURL string, data payloadData) (string, string, error) {
	if data.VerificationToken == "" {
		return "", "", fmt.Errorf("event %s has no verification token", eventType)
	}
	verificationURL, err := verificationURL(publicBaseURL, data.VerificationToken)
	if err != nil {
		return "", "", err
	}
	return "Verify your Identity Hub email", fmt.Sprintf("Hello %s,\n\nVerify your email address: %s\n", data.DisplayName, verificationURL), nil
}

func renderUserInvited(eventType, publicBaseURL string, data payloadData) (string, string, error) {
	if data.InvitationToken == "" {
		return "", "", fmt.Errorf("event %s has no invitation token", eventType)
	}
	acceptanceURL, err := invitationAcceptanceURL(publicBaseURL, data.InvitationToken)
	if err != nil {
		return "", "", err
	}
	return "You have been invited to Identity Hub", fmt.Sprintf("Hello %s,\n\nSet your password to accept your invitation: %s\n", data.DisplayName, acceptanceURL), nil
}

func renderPasswordResetRequested(eventType, publicBaseURL string, data payloadData) (string, string, error) {
	if data.ResetToken == "" {
		return "", "", fmt.Errorf("event %s has no reset token", eventType)
	}
	resetURL, err := accountLinkURL(publicBaseURL, "/password-reset", data.ResetToken)
	if err != nil {
		return "", "", err
	}
	// events.PasswordResetRequested never carries a display name (see
	// events.go): the request only ever knows the email, so a
	// personalized greeting here would be permanently dead code.
	return "Reset your Identity Hub password", fmt.Sprintf("Hello there,\n\nReset your password: %s\n", resetURL), nil
}

func renderPasswordResetCompleted(data payloadData) (string, string) {
	body := "Your password changed and your active sessions were closed."
	if data.Unlocked {
		body += " Your account was also unlocked."
	}
	body += "\n"
	return "Security alert: your Identity Hub password changed", body
}

func renderMfaChallengeIssued(eventType string, data payloadData) (string, string, error) {
	if data.Code == "" {
		return "", "", fmt.Errorf("event %s has no code", eventType)
	}
	return "Your Identity Hub sign-in code", fmt.Sprintf("Use this one-time code to finish signing in: %s\n", data.Code), nil
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
