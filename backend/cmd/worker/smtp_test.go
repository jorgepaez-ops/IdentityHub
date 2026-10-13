package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/config"
)

// testPKI is a throwaway CA plus a leaf for 127.0.0.1 so the SMTP client can
// verify a real chain instead of skipping verification.
type testPKI struct {
	caPEM []byte
	leaf  tls.Certificate
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test smtp ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return testPKI{
		caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		leaf:  tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey},
	}
}

func (p testPKI) writeCA(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, p.caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeSMTP is a minimal in-process SMTP server that records what it saw.
type fakeSMTP struct {
	ln            net.Listener
	tlsConfig     *tls.Config
	offerStartTLS bool
	implicitTLS   bool
	wantAuth      string // expected "user:pass"; empty = AUTH not offered
	dropAfterData bool   // close the connection right after accepting the message

	mu            sync.Mutex
	authBeforeTLS bool
	authOK        bool
	usedTLS       bool
	mailFrom      string
	rcpts         []string
	data          string
	commands      []string
}

func startFakeSMTP(t *testing.T, f *fakeSMTP) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f.ln = ln
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) record(cmd string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
}

// smtpSession is the per-connection state of the fake server: the (possibly upgraded)
// connection, its reader/writer and whether TLS is active.
type smtpSession struct {
	f         *fakeSMTP
	raw       net.Conn
	conn      net.Conn
	r         *textproto.Reader
	tlsActive bool
}

func (s *smtpSession) reply(line string) { _, _ = fmt.Fprintf(s.conn, "%s\r\n", line) }

// upgrade switches the session to TLS and records that it happened.
func (s *smtpSession) upgrade() {
	s.conn = tls.Server(s.raw, s.f.tlsConfig)
	s.r = textproto.NewReader(bufio.NewReader(s.conn))
	s.tlsActive = true
	s.f.mu.Lock()
	s.f.usedTLS = true
	s.f.mu.Unlock()
}

func (f *fakeSMTP) serve(raw net.Conn) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
	s := &smtpSession{f: f, raw: raw, conn: raw}
	if f.implicitTLS {
		s.upgrade() // wraps the connection in TLS and owns the reader setup
	} else {
		s.r = textproto.NewReader(bufio.NewReader(s.conn))
	}
	s.reply("220 fake ESMTP")
	for {
		line, err := s.r.ReadLine()
		if err != nil {
			return
		}
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		f.record(verb)
		if s.handle(verb, line) {
			return
		}
	}
}

// handle answers one command and reports whether the session is over.
func (s *smtpSession) handle(verb, line string) bool {
	f := s.f
	switch verb {
	case "EHLO", "HELO":
		s.reply("250-fake")
		if f.offerStartTLS && !s.tlsActive {
			s.reply("250-STARTTLS")
		}
		if f.wantAuth != "" {
			s.reply("250-AUTH PLAIN")
		}
		s.reply("250 8BITMIME")
	case "STARTTLS":
		s.reply("220 go ahead")
		s.upgrade()
	case "AUTH":
		s.auth(line)
	case "MAIL":
		f.mu.Lock()
		f.mailFrom = line
		f.mu.Unlock()
		s.reply("250 ok")
	case "RCPT":
		f.mu.Lock()
		f.rcpts = append(f.rcpts, line)
		f.mu.Unlock()
		s.reply("250 ok")
	case "DATA":
		s.reply("354 go")
		body, _ := s.r.ReadDotLines()
		f.mu.Lock()
		f.data = strings.Join(body, "\n")
		f.mu.Unlock()
		s.reply("250 queued")
		return f.dropAfterData
	case "QUIT":
		s.reply("221 bye")
		return true
	default:
		s.reply("250 ok")
	}
	return false
}

// auth handles AUTH PLAIN (initial response inline or in a 334 round trip) and records
// whether it arrived before TLS and whether the credentials matched.
func (s *smtpSession) auth(line string) {
	f := s.f
	if !s.tlsActive {
		f.mu.Lock()
		f.authBeforeTLS = true
		f.mu.Unlock()
	}
	parts := strings.Fields(line)
	raw64 := ""
	if len(parts) == 3 {
		raw64 = parts[2]
	} else {
		s.reply("334 ")
		raw64, _ = s.r.ReadLine()
	}
	decoded, _ := base64.StdEncoding.DecodeString(raw64)
	fields := strings.Split(string(decoded), "\x00")
	ok := len(fields) == 3 && fields[1]+":"+fields[2] == f.wantAuth
	f.mu.Lock()
	f.authOK = ok
	f.mu.Unlock()
	if ok {
		s.reply("235 ok")
	} else {
		s.reply("535 bad credentials")
	}
}

func (f *fakeSMTP) saw(verb string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.commands {
		if c == verb {
			return true
		}
	}
	return false
}

func mailerConfig(f *fakeSMTP, mutate func(*config.Config)) *config.Config {
	cfg := &config.Config{SMTPHost: "127.0.0.1", SMTPPort: f.port(), SMTPFrom: "no-reply@identity.local", SMTPTLS: "starttls"}
	if mutate != nil {
		mutate(cfg)
	}
	return cfg
}

func send(t *testing.T, cfg *config.Config) error {
	t.Helper()
	m, err := newSMTPMailer(cfg)
	if err != nil {
		return err
	}
	return m.Send(context.Background(), "no-reply@identity.local", []string{"ana@example.test"}, []byte("Subject: hi\r\n\r\nhello\r\n"))
}

func TestRF012_SMTPStartTLSObligatorioConAutenticacionEntregaElCorreo(t *testing.T) {
	pki := newTestPKI(t)
	f := &fakeSMTP{tlsConfig: &tls.Config{Certificates: []tls.Certificate{pki.leaf}, MinVersion: tls.VersionTLS12}, offerStartTLS: true, wantAuth: "hub:s3cret"}
	startFakeSMTP(t, f)

	err := send(t, mailerConfig(f, func(c *config.Config) {
		c.SMTPUsername = "hub"
		c.SMTPPassword = config.Secret("s3cret")
		c.SMTPCAFile = pki.writeCA(t)
	}))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.usedTLS || !f.authOK || f.authBeforeTLS {
		t.Fatalf("usedTLS=%v authOK=%v authBeforeTLS=%v", f.usedTLS, f.authOK, f.authBeforeTLS)
	}
	if !strings.Contains(f.mailFrom, "no-reply@identity.local") || len(f.rcpts) != 1 || !strings.Contains(f.data, "hello") {
		t.Fatalf("message not delivered: from=%q rcpts=%v data=%q", f.mailFrom, f.rcpts, f.data)
	}
}

func TestRF012_SMTPStartTLSObligatorioSinOfertaFallaSinEnviarNada(t *testing.T) {
	f := &fakeSMTP{offerStartTLS: false, wantAuth: "hub:s3cret"}
	startFakeSMTP(t, f)

	err := send(t, mailerConfig(f, func(c *config.Config) {
		c.SMTPUsername = "hub"
		c.SMTPPassword = config.Secret("s3cret")
	}))
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("Send() error = %v, want STARTTLS not offered", err)
	}
	sawAuth := f.saw("AUTH")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.authBeforeTLS || f.mailFrom != "" || f.data != "" || sawAuth {
		t.Fatalf("server saw cleartext traffic after refusal: %v", f.commands)
	}
}

func TestRF012_SMTPNoEnviaCredencialesSinTLS(t *testing.T) {
	// Even in "none" mode, credentials must never travel in cleartext.
	f := &fakeSMTP{wantAuth: "hub:s3cret"}
	startFakeSMTP(t, f)
	err := send(t, mailerConfig(f, func(c *config.Config) {
		c.SMTPTLS = "none"
		c.SMTPUsername = "hub"
		c.SMTPPassword = config.Secret("s3cret")
	}))
	if err == nil {
		t.Fatal("Send() succeeded sending credentials without TLS")
	}
	sawAuth := f.saw("AUTH")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.authBeforeTLS || sawAuth {
		t.Fatalf("AUTH was sent without TLS: %v", f.commands)
	}
}

func TestRF012_SMTPModoNoneEnviaSinTLS(t *testing.T) {
	f := &fakeSMTP{}
	startFakeSMTP(t, f)
	if err := send(t, mailerConfig(f, func(c *config.Config) { c.SMTPTLS = "none" })); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usedTLS || !strings.Contains(f.data, "hello") {
		t.Fatalf("usedTLS=%v data=%q", f.usedTLS, f.data)
	}
}

func TestRF012_SMTPQuitFallidoTrasAceptarElMensajeNoEsErrorDeEnvio(t *testing.T) {
	// The server already answered 250 to DATA: a lost QUIT must not make the worker requeue
	// the event and send the email twice.
	f := &fakeSMTP{dropAfterData: true}
	startFakeSMTP(t, f)
	if err := send(t, mailerConfig(f, func(c *config.Config) { c.SMTPTLS = "none" })); err != nil {
		t.Fatalf("Send() error = %v, want nil once the message was accepted", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.Contains(f.data, "hello") {
		t.Fatalf("data = %q", f.data)
	}
}

func TestRF012_SMTPCertificadoNoConfiableSinElCAFalla(t *testing.T) {
	pki := newTestPKI(t)
	f := &fakeSMTP{tlsConfig: &tls.Config{Certificates: []tls.Certificate{pki.leaf}, MinVersion: tls.VersionTLS12}, offerStartTLS: true}
	startFakeSMTP(t, f)
	err := send(t, mailerConfig(f, nil)) // no SMTP_CA_FILE: system roots do not trust the test CA
	if err == nil {
		t.Fatal("Send() accepted a certificate from an untrusted CA")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mailFrom != "" {
		t.Fatal("mail was sent over an unverified channel")
	}
}

func TestRF012_SMTPTLSImplicitoEntregaElCorreo(t *testing.T) {
	pki := newTestPKI(t)
	f := &fakeSMTP{tlsConfig: &tls.Config{Certificates: []tls.Certificate{pki.leaf}, MinVersion: tls.VersionTLS12}, implicitTLS: true, wantAuth: "hub:s3cret"}
	startFakeSMTP(t, f)
	err := send(t, mailerConfig(f, func(c *config.Config) {
		c.SMTPTLS = "tls"
		c.SMTPUsername = "hub"
		c.SMTPPassword = config.Secret("s3cret")
		c.SMTPCAFile = pki.writeCA(t)
	}))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.authOK || !strings.Contains(f.data, "hello") {
		t.Fatalf("authOK=%v data=%q", f.authOK, f.data)
	}
}

func TestRF012_SMTPCredencialesIncorrectasFallan(t *testing.T) {
	pki := newTestPKI(t)
	f := &fakeSMTP{tlsConfig: &tls.Config{Certificates: []tls.Certificate{pki.leaf}, MinVersion: tls.VersionTLS12}, offerStartTLS: true, wantAuth: "hub:s3cret"}
	startFakeSMTP(t, f)
	err := send(t, mailerConfig(f, func(c *config.Config) {
		c.SMTPUsername = "hub"
		c.SMTPPassword = config.Secret("wrong")
		c.SMTPCAFile = pki.writeCA(t)
	}))
	if err == nil {
		t.Fatal("Send() succeeded with wrong credentials")
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Fatalf("error leaks the password: %v", err)
	}
}

func TestRF012_SMTPArchivoCAInvalidoFallaAlConstruirElEnvio(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"sin PEM": bad, "inexistente": filepath.Join(t.TempDir(), "missing.pem")} {
		t.Run(name, func(t *testing.T) {
			_, err := newSMTPMailer(&config.Config{SMTPHost: "h", SMTPPort: 587, SMTPTLS: "starttls", SMTPCAFile: path})
			if err == nil {
				t.Fatal("newSMTPMailer() accepted an unusable CA file")
			}
		})
	}
}
