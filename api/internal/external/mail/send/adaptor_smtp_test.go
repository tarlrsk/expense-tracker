package send

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
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

const (
	testUser     = "mailer"
	testPassword = "smtp-test-password" //nolint:gosec // fake credentials for the in-process server
	recipient    = "person-7731@example.test"
)

// fakeServer is a tiny SMTP server for one test. It speaks just enough SMTP for go-mail.
type fakeServer struct {
	t             *testing.T
	ln            net.Listener
	tlsConfig     *tls.Config // server side; nil means no TLS at all
	implicitTLS   bool
	offerStartTLS bool
	offerAuth     bool
	silent        bool // accept connections but never greet
	rejectRcpt    bool

	mu       sync.Mutex
	open     []net.Conn
	messages []received
	commands []string // every command, with AUTH arguments replaced
}

type received struct {
	from, rcpt, data string
	tls              bool
	authUser         string
	authPassword     string
	authOverTLS      bool
}

func startServer(t *testing.T, setup func(*fakeServer)) *fakeServer {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{t: t, ln: ln}
	if setup != nil {
		setup(s)
	}
	done := make(chan struct{})
	var conns sync.WaitGroup
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.open = append(s.open, c)
			s.mu.Unlock()
			conns.Add(1)
			go func() { defer conns.Done(); s.serve(c) }()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
		s.mu.Lock()
		for _, c := range s.open {
			_ = c.Close()
		}
		s.mu.Unlock()
		conns.Wait()
	})
	return s
}

func (s *fakeServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeServer) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if s.silent {
		buf := make([]byte, 1)
		_, _ = conn.Read(buf) // wait for the client to give up
		return
	}
	isTLS := false
	if s.implicitTLS {
		conn = tls.Server(conn, s.tlsConfig)
		isTLS = true
	}
	r := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	readLine := func() (string, bool) {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}

	var cur received
	write("220 fake ESMTP")
	for {
		line, ok := readLine()
		if !ok {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		verb = strings.ToUpper(verb)
		s.record(verb, arg)
		switch verb {
		case "EHLO", "HELO":
			write("250-fake")
			if s.offerStartTLS && !isTLS {
				write("250-STARTTLS")
			}
			if s.offerAuth {
				write("250-AUTH PLAIN")
			}
			write("250 8BITMIME")
		case "STARTTLS":
			write("220 go ahead")
			tc := tls.Server(conn, s.tlsConfig)
			if err := tc.HandshakeContext(s.t.Context()); err != nil {
				return
			}
			conn, r, isTLS = tc, bufio.NewReader(tc), true
		case "AUTH":
			mech, initial, _ := strings.Cut(arg, " ")
			if !strings.EqualFold(mech, "PLAIN") {
				write("504 unsupported")
				continue
			}
			if initial == "" {
				write("334 ")
				if initial, ok = readLine(); !ok {
					return
				}
			}
			raw, err := base64.StdEncoding.DecodeString(initial)
			parts := strings.Split(string(raw), "\x00")
			if err != nil || len(parts) != 3 {
				write("501 bad auth")
				continue
			}
			cur.authUser, cur.authPassword, cur.authOverTLS = parts[1], parts[2], isTLS
			write("235 ok")
		case "MAIL":
			cur.from, cur.tls = arg, isTLS
			write("250 ok")
		case "RCPT":
			if s.rejectRcpt {
				write("550 5.1.1 no such user " + recipient)
				continue
			}
			cur.rcpt = arg
			write("250 ok")
		case "DATA":
			write("354 go ahead")
			var b strings.Builder
			for {
				l, ok := readLine()
				if !ok {
					return
				}
				if l == "." {
					break
				}
				b.WriteString(l + "\n")
			}
			cur.data = b.String()
			s.mu.Lock()
			s.messages = append(s.messages, cur)
			s.mu.Unlock()
			cur = received{authUser: cur.authUser, authPassword: cur.authPassword, authOverTLS: cur.authOverTLS}
			write("250 queued")
		case "RSET", "NOOP":
			write("250 ok")
		case "QUIT":
			write("221 bye")
			return
		default:
			write("502 not implemented")
		}
	}
}

func (s *fakeServer) record(verb, arg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if verb == "AUTH" {
		arg = "[hidden]"
	}
	s.commands = append(s.commands, strings.TrimSpace(verb+" "+arg))
}

func (s *fakeServer) received() ([]received, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]received(nil), s.messages...), append([]string(nil), s.commands...)
}

// testCert makes a self-signed certificate for 127.0.0.1 and returns the server and client TLS
// configurations.
func testCert(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "fake smtp"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	server = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}},
		MinVersion:   tls.VersionTLS12,
	}
	client = &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	return server, client
}

func smtpConfig(port int, mode config.SMTPTLS, withAuth bool) config.SMTP {
	c := config.SMTP{Host: "127.0.0.1", Port: port, From: "Satang <noreply@example.test>", TLS: mode}
	if withAuth {
		c.Username, c.Password = testUser, testPassword
	}
	return c
}

var testMessage = Message{To: recipient, Subject: "Your link", Text: "Open this link to set your password."}

func TestSMTPSends(t *testing.T) {
	serverTLS, clientTLS := testCert(t)
	tests := []struct {
		name     string
		mode     config.SMTPTLS
		withAuth bool
		setup    func(*fakeServer)
		wantTLS  bool
	}{
		{name: "none (Mailpit)", mode: config.SMTPTLSNone},
		{
			name: "starttls with auth", mode: config.SMTPTLSStartTLS, withAuth: true, wantTLS: true,
			setup: func(s *fakeServer) { s.tlsConfig, s.offerStartTLS, s.offerAuth = serverTLS, true, true },
		},
		{
			name: "implicit tls with auth", mode: config.SMTPTLSImplicit, withAuth: true, wantTLS: true,
			setup: func(s *fakeServer) { s.tlsConfig, s.implicitTLS, s.offerAuth = serverTLS, true, true },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := startServer(t, tt.setup)
			p, err := newSMTP(smtpConfig(srv.port(), tt.mode, tt.withAuth), clientTLS)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Send(t.Context(), testMessage); err != nil {
				t.Fatalf("Send: %v", err)
			}
			msgs, _ := srv.received()
			if len(msgs) != 1 {
				t.Fatalf("server got %d messages, want 1", len(msgs))
			}
			got := msgs[0]
			if !strings.Contains(got.from, "noreply@example.test") || !strings.Contains(got.rcpt, recipient) {
				t.Errorf("envelope from %q to %q", got.from, got.rcpt)
			}
			for _, want := range []string{"Subject: Your link", "Open this link to set your password.", "Content-Type: text/plain"} {
				if !strings.Contains(got.data, want) {
					t.Errorf("message lacks %q:\n%s", want, got.data)
				}
			}
			if got.tls != tt.wantTLS {
				t.Errorf("sent over TLS = %v, want %v", got.tls, tt.wantTLS)
			}
			if tt.withAuth && (got.authUser != testUser || got.authPassword != testPassword || !got.authOverTLS) {
				t.Errorf("auth user %q over TLS %v; want %q over TLS", got.authUser, got.authOverTLS, testUser)
			}
		})
	}
}

// Under starttls a server without STARTTLS is refused before any credential or message is sent.
func TestSMTPStartTLSIsMandatory(t *testing.T) {
	srv := startServer(t, func(s *fakeServer) { s.offerAuth = true })
	p, err := NewSMTP(smtpConfig(srv.port(), config.SMTPTLSStartTLS, true))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Send(t.Context(), testMessage); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("Send = %v, want a STARTTLS error", err)
	}
	msgs, cmds := srv.received()
	if len(msgs) != 0 {
		t.Errorf("server got %d messages", len(msgs))
	}
	for _, c := range cmds {
		if strings.HasPrefix(c, "AUTH") || strings.HasPrefix(c, "MAIL") {
			t.Errorf("client sent %q over a clear connection", c)
		}
	}
}

// Send stops at the context's deadline even when the server never answers.
func TestSMTPHonoursDeadline(t *testing.T) {
	srv := startServer(t, func(s *fakeServer) { s.silent = true })
	p, err := NewSMTP(smtpConfig(srv.port(), config.SMTPTLSNone, false))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = p.Send(ctx, testMessage)
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("Send took %s, want it to stop near the 300ms deadline", took)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Send = %v, want context.DeadlineExceeded", err)
	}

	// A context already past its deadline does not even connect.
	expired, cancel2 := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel2()
	if err := p.Send(expired, testMessage); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Send on an expired context = %v, want context.DeadlineExceeded", err)
	}
}

// Send is an outside call: inside an open transaction it refuses without connecting (ADR-0032).
func TestSMTPRefusesInsideTransaction(t *testing.T) {
	srv := startServer(t, nil)
	p, err := NewSMTP(smtpConfig(srv.port(), config.SMTPTLSNone, false))
	if err != nil {
		t.Fatal(err)
	}
	var sendErr error
	_ = txtest.New().WithUserTx(t.Context(), uuid.New(), func(ctx context.Context) error {
		sendErr = p.Send(ctx, testMessage)
		return nil
	})
	if !errors.Is(sendErr, tx.ErrInside) {
		t.Errorf("Send inside a transaction = %v, want tx.ErrInside", sendErr)
	}
	if _, cmds := srv.received(); len(cmds) != 0 {
		t.Errorf("client talked to the server: %v", cmds)
	}
}

// A delivery error names the reason and code, never the recipient.
func TestSMTPErrorHidesRecipient(t *testing.T) {
	srv := startServer(t, func(s *fakeServer) { s.rejectRcpt = true })
	p, err := NewSMTP(smtpConfig(srv.port(), config.SMTPTLSNone, false))
	if err != nil {
		t.Fatal(err)
	}
	err = p.Send(t.Context(), testMessage)
	if err == nil {
		t.Fatal("Send succeeded, want a rejected recipient")
	}
	if strings.Contains(err.Error(), recipient) || !strings.Contains(err.Error(), "550") {
		t.Errorf("error = %q; want the SMTP code without the recipient", err)
	}
	if err := p.Send(t.Context(), Message{To: "not an address " + recipient}); err == nil ||
		strings.Contains(err.Error(), recipient) {
		t.Errorf("bad recipient error = %v; want an error without the address", err)
	}
}

func TestNewSMTPRefuses(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.SMTP
		wantErr string
	}{
		{name: "credentials over a clear connection", cfg: smtpConfig(1025, config.SMTPTLSNone, true), wantErr: "clear connection"},
		{
			name:    "username without password",
			cfg:     config.SMTP{Host: "127.0.0.1", Port: 587, From: "a@example.test", TLS: config.SMTPTLSStartTLS, Username: "u"},
			wantErr: "both",
		},
		{name: "unknown tls mode", cfg: smtpConfig(1025, "opportunistic", false), wantErr: "unknown TLS mode"},
		{name: "empty host", cfg: config.SMTP{Port: 1025, From: "a@example.test", TLS: config.SMTPTLSNone}, wantErr: "host"},
		{name: "bad port", cfg: smtpConfig(0, config.SMTPTLSNone, false), wantErr: "port"},
		{
			name:    "bad sender",
			cfg:     config.SMTP{Host: "127.0.0.1", Port: 1025, From: "nobody", TLS: config.SMTPTLSNone},
			wantErr: "sender",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSMTP(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewSMTP = %v, want an error mentioning %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), testPassword) {
				t.Errorf("error leaks the password: %v", err)
			}
		})
	}
}
