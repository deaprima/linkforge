package mailer

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP adalah server SMTP minimal untuk menguji SMTPMailer lewat jaringan sungguhan.
type fakeSMTP struct {
	ln net.Listener

	advertiseAuth bool // tawarkan AUTH PLAIN
	rejectAuth    bool // jawab 535 untuk AUTH
	rejectRcpt    bool // jawab 550 untuk RCPT TO
	dropAtData    bool // putus koneksi saat DATA

	mu       sync.Mutex
	mailFrom string
	rcptTo   string
	data     string
	authed   bool
}

func newFakeSMTP(t *testing.T, configure func(*fakeSMTP)) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSMTP{ln: ln}
	if configure != nil {
		configure(s)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeSMTP) addr() (host, port string) {
	host, port, _ = net.SplitHostPort(s.ln.Addr().String())
	return
}

func (s *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(line string) { conn.Write([]byte(line + "\r\n")) }
	w("220 fake ESMTP ready")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			if s.advertiseAuth {
				w("250-fake")
				w("250 AUTH PLAIN")
			} else {
				w("250 fake")
			}
		case strings.HasPrefix(cmd, "AUTH"):
			if s.rejectAuth || !s.advertiseAuth { // server asli menolak AUTH yang tidak diiklankan
				w("535 authentication failed")
				continue
			}
			s.mu.Lock()
			s.authed = true
			s.mu.Unlock()
			w("235 ok")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			s.mu.Lock()
			s.mailFrom = strings.TrimSpace(line)
			s.mu.Unlock()
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			if s.rejectRcpt {
				w("550 no such user")
				continue
			}
			s.mu.Lock()
			s.rcptTo = strings.TrimSpace(line)
			s.mu.Unlock()
			w("250 ok")
		case cmd == "DATA":
			if s.dropAtData {
				return
			}
			w("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			s.mu.Lock()
			s.data = b.String()
			s.mu.Unlock()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("502 not implemented")
		}
	}
}

func newMailer(s *fakeSMTP, user, pass string) *SMTPMailer {
	host, port := s.addr()
	return &SMTPMailer{
		cfg:     SMTPConfig{Host: host, Port: port, User: user, Pass: pass, From: "LinkForge <no-reply@linkforge.test>"},
		timeout: 3 * time.Second,
	}
}

var testMessage = Message{To: "dea@example.com", Subject: "Reset your password", Text: "plain body", HTML: "<p>html body</p>"}

func TestSMTPMailer_SendDeliversMessage(t *testing.T) {
	s := newFakeSMTP(t, nil)
	if err := newMailer(s, "", "").Send(context.Background(), testMessage); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.Contains(s.mailFrom, "no-reply@linkforge.test") || !strings.Contains(s.rcptTo, "dea@example.com") {
		t.Errorf("envelope: from=%q to=%q", s.mailFrom, s.rcptTo)
	}
	for _, want := range []string{"Subject: Reset your password", "To: <dea@example.com>", "plain body", "<p>html body</p>", "multipart/alternative"} {
		if !strings.Contains(s.data, want) {
			t.Errorf("message lacks %q:\n%s", want, s.data)
		}
	}
	if s.authed {
		t.Error("must not authenticate when SMTP_USER is empty")
	}
}

func TestSMTPMailer_AuthenticatesWhenCredentialsGiven(t *testing.T) {
	s := newFakeSMTP(t, func(s *fakeSMTP) { s.advertiseAuth = true })
	if err := newMailer(s, "mailer", "pw").Send(context.Background(), testMessage); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.authed {
		t.Error("expected AUTH to be performed")
	}
}

func TestSMTPMailer_Failures(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*fakeSMTP)
		user      string
		wantErr   string
	}{
		{"auth rejected", func(s *fakeSMTP) { s.advertiseAuth, s.rejectAuth = true, true }, "mailer", "smtp auth"},
		{"auth requested but not offered", nil, "mailer", "smtp auth"},
		{"recipient rejected", func(s *fakeSMTP) { s.rejectRcpt = true }, "", "smtp RCPT TO"},
		{"connection dropped during DATA", func(s *fakeSMTP) { s.dropAtData = true }, "", "smtp DATA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newFakeSMTP(t, tc.configure)
			err := newMailer(s, tc.user, "pw").Send(context.Background(), testMessage)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestSMTPMailer_DialFailure(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	ln.Close() // port ditutup → koneksi ditolak

	m := &SMTPMailer{cfg: SMTPConfig{Host: host, Port: port, From: "a@b.test"}, timeout: time.Second}
	err := m.Send(context.Background(), testMessage)
	if err == nil || !strings.Contains(err.Error(), "smtp dial") {
		t.Errorf("err = %v, want smtp dial error", err)
	}
}

func TestSMTPMailer_HandshakeFailure(t *testing.T) {
	// Server yang langsung menutup koneksi tanpa greeting
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			c.Close()
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())

	m := &SMTPMailer{cfg: SMTPConfig{Host: host, Port: port, From: "a@b.test"}, timeout: time.Second}
	err = m.Send(context.Background(), testMessage)
	if err == nil || !strings.Contains(err.Error(), "smtp handshake") {
		t.Errorf("err = %v, want smtp handshake error", err)
	}
}

func TestSMTPMailer_RespectsContextCancellation(t *testing.T) {
	s := newFakeSMTP(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newMailer(s, "", "").Send(ctx, testMessage); err == nil {
		t.Error("cancelled context must fail the send")
	}
}

func TestLogMailer_Send(t *testing.T) {
	if err := (LogMailer{}).Send(context.Background(), testMessage); err != nil {
		t.Errorf("LogMailer must never fail: %v", err)
	}
}
