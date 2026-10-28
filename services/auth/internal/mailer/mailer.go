// Package mailer mengirim email transaksional Auth Service (saat ini hanya reset password).
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Message adalah email multipart (teks polos + HTML).
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPConfig diisi dari env SMTP_*. Host kosong = SMTP tidak dikonfigurasi (pakai LogMailer).
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string // mis. "LinkForge <no-reply@example.com>"
}

func (c SMTPConfig) Enabled() bool { return c.Host != "" }

// New mengembalikan SMTPMailer kalau dikonfigurasi, kalau tidak LogMailer (hanya cocok untuk development).
func New(cfg SMTPConfig) Mailer {
	if !cfg.Enabled() {
		log.Println("Warning: SMTP_HOST not set, emails are only written to the log (development mode)")
		return LogMailer{}
	}
	return &SMTPMailer{cfg: cfg, timeout: 15 * time.Second}
}

// LogMailer mencetak email ke log supaya alur reset password bisa dicoba lokal tanpa SMTP.
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, msg Message) error {
	log.Printf("[mailer] (SMTP not configured) to=%s subject=%q\n%s", msg.To, msg.Subject, msg.Text)
	return nil
}

type SMTPMailer struct {
	cfg     SMTPConfig
	timeout time.Duration
}

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	from, err := mail.ParseAddress(m.cfg.From)
	if err != nil {
		return fmt.Errorf("invalid SMTP_FROM %q: %w", m.cfg.From, err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("invalid recipient %q: %w", msg.To, err)
	}

	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	dialer := &net.Dialer{Timeout: m.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(m.timeout))

	// Port 465 = TLS sejak awal (implicit); port lain memakai STARTTLS kalau server menawarkan.
	if m.cfg.Port == "465" {
		conn = tls.Client(conn, &tls.Config{ServerName: m.cfg.Host})
	}
	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer client.Close()

	if m.cfg.Port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if m.cfg.User != "" {
		if err := client.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(buildMIME(from, to, msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

// buildMIME merakit email multipart/alternative. Header diperiksa supaya input tidak bisa menyisipkan header lain.
func buildMIME(from, to *mail.Address, msg Message) []byte {
	const boundary = "linkforge-boundary-7f3a9c"
	var b strings.Builder
	h := func(k, v string) { b.WriteString(k + ": " + sanitizeHeader(v) + "\r\n") }
	h("From", from.String())
	h("To", to.String())
	h("Subject", mimeSubject(msg.Subject))
	h("Date", time.Now().UTC().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")

	part := func(contentType, body string) {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + contentType + "; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(strings.ReplaceAll(body, "\n", "\r\n") + "\r\n")
	}
	part("text/plain", msg.Text)
	part("text/html", msg.HTML)
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(v)
}

// mimeSubject meng-encode subjek non-ASCII (RFC 2047) supaya tampil benar di semua klien.
func mimeSubject(s string) string {
	return mime.QEncoding.Encode("UTF-8", s)
}
