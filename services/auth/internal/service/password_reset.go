package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/mailer"
	"github.com/deaprima/linkforge/services/auth/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidResetToken = token tidak dikenal, kedaluwarsa, atau sudah dipakai (sengaja satu pesan, supaya tidak jadi oracle)
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
	// ErrInvalidPassword = password baru di luar batas 8-72 byte (batas bcrypt)
	ErrInvalidPassword = errors.New("password must be between 8 and 72 characters")
)

const (
	// Maksimal email reset per akun per jam; sisanya diabaikan diam-diam supaya akun korban tidak dibanjiri email.
	maxResetEmailsPerHour = 3
	resetEmailWindow      = time.Hour
	minPasswordLen        = 8
	maxPasswordLen        = 72
)

// PasswordResetConfig = dependensi alur lupa password.
type PasswordResetConfig struct {
	Repo        repository.PasswordResetRepository
	Mailer      mailer.Mailer
	FrontendURL string        // basis link di email, mis. https://linkforge.deaprima.my.id
	TTL         time.Duration // masa berlaku token
}

// RequestPasswordReset membuat token dan mengirim emailnya. Selalu nil untuk email yang tidak terdaftar
// (handler membalas sama persis), hanya error infrastruktur yang dikembalikan.
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}

	recent, err := s.reset.Repo.CountSince(ctx, user.ID, time.Now().Add(-resetEmailWindow))
	if err != nil {
		return err
	}
	if recent >= maxResetEmailsPerHour {
		log.Printf("[auth] password reset throttled for user %s (%d requests in the last hour)", user.ID, recent)
		return nil
	}

	raw, err := generateResetToken()
	if err != nil {
		return err
	}
	// Permintaan baru mematikan link lama yang belum dipakai
	if err := s.reset.Repo.InvalidateAllByUserID(ctx, user.ID); err != nil {
		return err
	}
	err = s.reset.Repo.Create(ctx, &entity.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: time.Now().Add(s.reset.TTL),
	})
	if err != nil {
		return err
	}

	msg := resetEmail(user.Name, user.Email, s.resetLink(raw), s.reset.TTL)
	// Dikirim di background supaya lama-tidaknya SMTP tidak membedakan email terdaftar dari yang tidak
	s.async(func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.reset.Mailer.Send(sendCtx, msg); err != nil {
			log.Printf("[auth] failed to send password reset email to user %s: %v", user.ID, err)
		}
	})
	return nil
}

// ResetPassword menyetel password baru dan mencabut semua sesi user.
func (s *authService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < minPasswordLen || len(newPassword) > maxPasswordLen {
		return ErrInvalidPassword
	}

	token, err := s.reset.Repo.GetByHash(ctx, hashToken(strings.TrimSpace(rawToken)))
	if err != nil {
		return err
	}
	if token == nil || token.UsedAt != nil || time.Now().After(token.ExpiresAt) {
		return ErrInvalidResetToken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Consume atomik: dua request paralel dengan token sama, hanya satu yang menang
	won, err := s.reset.Repo.Consume(ctx, token.ID)
	if err != nil {
		return err
	}
	if !won {
		return ErrInvalidResetToken
	}

	if err := s.userRepo.UpdatePassword(ctx, token.UserID, string(hashed)); err != nil {
		return err
	}
	// Password berubah = semua sesi lama (mungkin dicuri) harus mati, juga link reset lain yang masih menggantung
	if err := s.tokenRepo.RevokeAllByUserID(ctx, token.UserID); err != nil {
		return err
	}
	return s.reset.Repo.InvalidateAllByUserID(ctx, token.UserID)
}

func (s *authService) resetLink(raw string) string {
	return strings.TrimRight(s.reset.FrontendURL, "/") + "/app/reset-password?token=" + url.QueryEscape(raw)
}

// generateResetToken = 32 byte acak (256 bit), hex.
func generateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate reset token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func resetEmail(name, to, link string, ttl time.Duration) mailer.Message {
	validFor := humanDuration(ttl)
	text := fmt.Sprintf(`Hi %s,

Someone asked to reset the password for your LinkForge account. To choose a new one, open this link:

%s

The link works once and expires in %s. If you didn't ask for this, you can ignore this email; your password stays the same.

LinkForge`, name, link, validFor)

	htmlBody := fmt.Sprintf(`<!doctype html>
<html><body style="margin:0;padding:24px;background:#ffffff;font-family:Arial,Helvetica,sans-serif;color:#000000;">
<div style="max-width:480px;margin:0 auto;">
<p style="font-size:18px;font-weight:bold;margin:0 0 24px;">linkforge</p>
<p style="font-size:15px;line-height:1.5;margin:0 0 16px;">Hi %s,</p>
<p style="font-size:15px;line-height:1.5;margin:0 0 24px;">Someone asked to reset the password for your LinkForge account. To choose a new one, use the button below.</p>
<p style="margin:0 0 24px;"><a href="%s" style="display:inline-block;background:#000000;color:#ffffff;text-decoration:none;font-weight:bold;font-size:15px;padding:12px 20px;">Reset password</a></p>
<p style="font-size:13px;line-height:1.5;color:#5c5c5c;margin:0 0 8px;">The link works once and expires in %s. If the button doesn't work, paste this address into your browser:</p>
<p style="font-size:12px;line-height:1.5;color:#5c5c5c;word-break:break-all;margin:0 0 24px;">%s</p>
<p style="font-size:13px;line-height:1.5;color:#5c5c5c;margin:0;">If you didn't ask for this, you can ignore this email; your password stays the same.</p>
</div></body></html>`, html.EscapeString(name), html.EscapeString(link), validFor, html.EscapeString(link))

	return mailer.Message{To: to, Subject: "Reset your LinkForge password", Text: text, HTML: htmlBody}
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		h := int(d / time.Hour)
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	default:
		m := int(d / time.Minute)
		if m == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", m)
	}
}
