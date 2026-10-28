package mailer

import (
	"context"
	"net/mail"
	"strings"
	"testing"
)

func TestNew_FallsBackToLogMailerWithoutSMTP(t *testing.T) {
	m := New(SMTPConfig{})
	if _, ok := m.(LogMailer); !ok {
		t.Fatalf("want LogMailer, got %T", m)
	}
	if err := m.Send(context.Background(), Message{To: "a@b.c", Subject: "s", Text: "t"}); err != nil {
		t.Fatalf("LogMailer must not fail: %v", err)
	}
	if _, ok := New(SMTPConfig{Host: "smtp.example.com", Port: "587"}).(*SMTPMailer); !ok {
		t.Fatal("want SMTPMailer when SMTP_HOST is set")
	}
}

func TestBuildMIME_MultipartAndHeaders(t *testing.T) {
	from, _ := mail.ParseAddress("LinkForge <no-reply@example.com>")
	to, _ := mail.ParseAddress("dea@example.com")
	raw := string(buildMIME(from, to, Message{Subject: "Reset your password", Text: "plain body", HTML: "<p>html body</p>"}))

	for _, want := range []string{
		"From: \"LinkForge\" <no-reply@example.com>\r\n",
		"To: <dea@example.com>\r\n",
		"Subject: Reset your password\r\n",
		"MIME-Version: 1.0\r\n",
		"multipart/alternative",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"plain body",
		"<p>html body</p>",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("missing %q in:\n%s", want, raw)
		}
	}
	// Bagian teks harus lebih dulu dari HTML (klien memilih bagian terakhir yang didukung)
	if strings.Index(raw, "text/plain") > strings.Index(raw, "text/html") {
		t.Error("text/plain must come before text/html")
	}
}

func TestBuildMIME_SubjectCannotInjectHeaders(t *testing.T) {
	from, _ := mail.ParseAddress("no-reply@example.com")
	to, _ := mail.ParseAddress("dea@example.com")
	raw := string(buildMIME(from, to, Message{Subject: "Hi\r\nBcc: victim@example.com", Text: "t", HTML: "h"}))
	header := raw[:strings.Index(raw, "\r\n\r\n")]
	if strings.Contains(header, "\r\nBcc:") {
		t.Fatalf("subject injected a header:\n%s", header)
	}
}

func TestMimeSubject_EncodesNonASCII(t *testing.T) {
	if got := mimeSubject("Plain"); got != "Plain" {
		t.Errorf("ASCII subject changed: %q", got)
	}
	if got := mimeSubject("Atur ulang kata sandi — LinkForge"); !strings.HasPrefix(got, "=?UTF-8?q?") {
		t.Errorf("non-ASCII subject not RFC 2047 encoded: %q", got)
	}
}

func TestSMTPMailer_RejectsBadAddresses(t *testing.T) {
	m := &SMTPMailer{cfg: SMTPConfig{Host: "127.0.0.1", Port: "1", From: "not an address"}}
	if err := m.Send(context.Background(), Message{To: "dea@example.com"}); err == nil || !strings.Contains(err.Error(), "SMTP_FROM") {
		t.Fatalf("want SMTP_FROM error, got %v", err)
	}
	m.cfg.From = "no-reply@example.com"
	if err := m.Send(context.Background(), Message{To: "bad recipient"}); err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("want recipient error, got %v", err)
	}
}
