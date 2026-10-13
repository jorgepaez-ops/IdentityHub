package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/jorgepaez/identity-hub/internal/config"
)

const (
	smtpDialTimeout = 10 * time.Second
	// smtpSessionTimeout bounds one whole SMTP conversation: net/smtp has no
	// context, so a stuck server would otherwise block the consumer forever.
	smtpSessionTimeout = 30 * time.Second
)

// smtpMailer delivers one message per connection with an explicit net/smtp
// flow instead of smtp.SendMail, so transport security is enforced rather
// than opportunistic: STARTTLS is required (or implicit TLS used), TLS 1.2 is
// the floor, the certificate is verified against SMTP_HOST, and credentials
// are only ever sent over an encrypted channel.
type smtpMailer struct {
	host, addr     string
	username       string
	credential     string
	mode           string
	rootCAs        *x509.CertPool // nil = system roots
	dialTimeout    time.Duration
	sessionTimeout time.Duration
}

// newSMTPMailer validates the settings and loads SMTP_CA_FILE once, so a bad
// bundle fails at startup instead of on the first notification.
func newSMTPMailer(cfg *config.Config) (*smtpMailer, error) {
	mode := cfg.SMTPTLS
	if mode == "" {
		mode = config.SMTPTLSStartTLS
	}
	m := &smtpMailer{
		host:           cfg.SMTPHost,
		addr:           net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort)),
		username:       cfg.SMTPUsername,
		credential:     cfg.SMTPPassword.Reveal(),
		mode:           mode,
		dialTimeout:    smtpDialTimeout,
		sessionTimeout: smtpSessionTimeout,
	}
	if cfg.SMTPCAFile != "" {
		pemBytes, err := os.ReadFile(cfg.SMTPCAFile)
		if err != nil {
			return nil, fmt.Errorf("leyendo SMTP_CA_FILE: %w", err)
		}
		// Extend the system roots rather than replace them, so a bundle with
		// only the local CA still lets a public SMTP relay verify.
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, errors.New("SMTP_CA_FILE no contiene ningún certificado PEM válido")
		}
		m.rootCAs = pool
	}
	return m, nil
}

func (m *smtpMailer) tlsConfig() *tls.Config {
	return &tls.Config{ServerName: m.host, RootCAs: m.rootCAs, MinVersion: tls.VersionTLS12}
}

// Send delivers message to the recipients. Errors never include the password.
func (m *smtpMailer) Send(ctx context.Context, from string, to []string, message []byte) error {
	dialer := &net.Dialer{Timeout: m.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("conectando con el servidor SMTP: %w", err)
	}
	deadline := time.Now().Add(m.sessionTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	if m.mode == config.SMTPTLSImplicit {
		conn = tls.Client(conn, m.tlsConfig())
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("saludo SMTP: %w", err)
	}
	defer func() { _ = client.Close() }()

	encrypted := m.mode == config.SMTPTLSImplicit
	if m.mode == config.SMTPTLSStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("el servidor SMTP no ofrece STARTTLS y SMTP_TLS=starttls lo exige")
		}
		if err := client.StartTLS(m.tlsConfig()); err != nil {
			return fmt.Errorf("negociando STARTTLS: %w", err)
		}
		encrypted = true
	}

	if m.username != "" {
		if !encrypted {
			return errors.New("se niega a autenticarse por SMTP sin TLS")
		}
		if ok, _ := client.Extension("AUTH"); !ok {
			return errors.New("el servidor SMTP no ofrece AUTH")
		}
		if err := client.Auth(smtp.PlainAuth("", m.username, m.credential, m.host)); err != nil {
			return fmt.Errorf("autenticación SMTP rechazada: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(message); err != nil {
		_ = w.Close()
		return fmt.Errorf("escribiendo el mensaje: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("cerrando DATA: %w", err)
	}
	// The server already accepted the message (250 to DATA). A failed QUIT (deadline, reset)
	// must not be reported as a send failure: the worker would requeue the event and send the
	// email twice. It is best-effort; Close releases the connection either way.
	if err := client.Quit(); err != nil {
		_ = client.Close()
	}
	return nil
}
