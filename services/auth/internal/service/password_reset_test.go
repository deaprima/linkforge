package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/mailer"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type fakeResetRepo struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]*entity.PasswordResetToken
	err    error
}

func newFakeResetRepo() *fakeResetRepo {
	return &fakeResetRepo{tokens: map[uuid.UUID]*entity.PasswordResetToken{}}
}

func (r *fakeResetRepo) Create(_ context.Context, t *entity.PasswordResetToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	r.tokens[t.ID] = t
	return nil
}

func (r *fakeResetRepo) GetByHash(_ context.Context, hash string) (*entity.PasswordResetToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, nil
}

func (r *fakeResetRepo) Consume(_ context.Context, id uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.tokens[id]
	if t.UsedAt != nil {
		return false, nil
	}
	now := time.Now()
	t.UsedAt = &now
	return true, nil
}

func (r *fakeResetRepo) InvalidateAllByUserID(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, t := range r.tokens {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &now
		}
	}
	return nil
}

func (r *fakeResetRepo) CountSince(_ context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, t := range r.tokens {
		if t.UserID == userID && !t.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

// usable = token yang masih bisa dipakai
func (r *fakeResetRepo) usable(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, t := range r.tokens {
		if t.UserID == userID && t.UsedAt == nil && time.Now().Before(t.ExpiresAt) {
			n++
		}
	}
	return n
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []mailer.Message
	err  error
}

func (m *fakeMailer) Send(_ context.Context, msg mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

// tokenFromMail mengambil token mentah dari link di email terakhir.
func tokenFromMail(t *testing.T, m *fakeMailer) string {
	t.Helper()
	if len(m.sent) == 0 {
		t.Fatal("no email was sent")
	}
	text := m.sent[len(m.sent)-1].Text
	start := strings.Index(text, "https://")
	if start < 0 {
		t.Fatalf("no link in email:\n%s", text)
	}
	link := strings.Fields(text[start:])[0]
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("token")
}

func TestRequestPasswordReset_SendsLink(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)

	if err := env.svc.RequestPasswordReset(context.Background(), "  Dea@Example.com "); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if len(env.mail.sent) != 1 {
		t.Fatalf("want 1 email, got %d", len(env.mail.sent))
	}
	msg := env.mail.sent[0]
	if msg.To != "dea@example.com" || !strings.Contains(msg.Subject, "Reset") {
		t.Fatalf("unexpected email: to=%q subject=%q", msg.To, msg.Subject)
	}
	if !strings.Contains(msg.Text, "https://app.example.com/app/reset-password?token=") || !strings.Contains(msg.HTML, "/app/reset-password?token=") {
		t.Fatalf("email has no reset link:\n%s", msg.Text)
	}
	// Hanya hash yang disimpan, bukan token mentah
	raw := tokenFromMail(t, env.mail)
	if len(raw) != 64 {
		t.Fatalf("want 64 hex chars, got %d", len(raw))
	}
	for _, tok := range env.resets.tokens {
		if tok.TokenHash == raw || tok.TokenHash != hashToken(raw) {
			t.Fatal("token must be stored hashed")
		}
	}
}

func TestRequestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	env := newTestEnv()
	if err := env.svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("want nil for unknown email, got %v", err)
	}
	if len(env.mail.sent) != 0 || len(env.resets.tokens) != 0 {
		t.Fatal("nothing should be created or sent for an unknown email")
	}
}

func TestRequestPasswordReset_NewRequestInvalidatesOldLink(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	ctx := context.Background()

	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	first := tokenFromMail(t, env.mail)
	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	second := tokenFromMail(t, env.mail)

	if env.resets.usable(user.ID) != 1 {
		t.Fatalf("want exactly 1 usable token, got %d", env.resets.usable(user.ID))
	}
	if err := env.svc.ResetPassword(ctx, first, "brandnewpass"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("old link must stop working, got %v", err)
	}
	if err := env.svc.ResetPassword(ctx, second, "brandnewpass"); err != nil {
		t.Fatalf("newest link must work: %v", err)
	}
}

func TestRequestPasswordReset_ThrottlesPerAccount(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	for i := 0; i < maxResetEmailsPerHour+2; i++ {
		if err := env.svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if len(env.mail.sent) != maxResetEmailsPerHour {
		t.Fatalf("want %d emails, got %d", maxResetEmailsPerHour, len(env.mail.sent))
	}
}

func TestRequestPasswordReset_MailerFailureDoesNotFailRequest(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	env.mail.err = errors.New("smtp down")
	if err := env.svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("a failed email must not change the response (no account oracle), got %v", err)
	}
}

func TestResetPassword_ChangesPasswordAndRevokesSessions(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	ctx := context.Background()

	// Sesi aktif sebelum reset
	if _, _, err := env.svc.Login(ctx, LoginInput{Email: user.Email, Password: "oldpassword"}); err != nil {
		t.Fatal(err)
	}
	if env.tokens.active(user.ID) != 1 {
		t.Fatal("expected one active session")
	}

	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	raw := tokenFromMail(t, env.mail)
	if err := env.svc.ResetPassword(ctx, raw, "brandnewpass"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte("brandnewpass")); err != nil {
		t.Fatal("new password must be stored")
	}
	if _, _, err := env.svc.Login(ctx, LoginInput{Email: user.Email, Password: "oldpassword"}); err == nil {
		t.Fatal("old password must stop working")
	}
	if _, _, err := env.svc.Login(ctx, LoginInput{Email: user.Email, Password: "brandnewpass"}); err != nil {
		t.Fatalf("new password must work: %v", err)
	}
	// Login terakhir membuat 1 sesi baru; sesi lama harus sudah dicabut
	if got := env.tokens.active(user.ID); got != 1 {
		t.Fatalf("old sessions must be revoked, active=%d", got)
	}
}

func TestResetPassword_TokenIsSingleUse(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	ctx := context.Background()
	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	raw := tokenFromMail(t, env.mail)

	if err := env.svc.ResetPassword(ctx, raw, "brandnewpass"); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.ResetPassword(ctx, raw, "anotherpass1"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("second use must fail, got %v", err)
	}
}

func TestResetPassword_ExpiredOrUnknownToken(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	ctx := context.Background()

	if err := env.svc.ResetPassword(ctx, "does-not-exist", "brandnewpass"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("unknown token: %v", err)
	}

	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	raw := tokenFromMail(t, env.mail)
	for _, tok := range env.resets.tokens {
		tok.ExpiresAt = time.Now().Add(-time.Minute)
	}
	if err := env.svc.ResetPassword(ctx, raw, "brandnewpass"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expired token: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte("oldpassword")); err != nil {
		t.Fatal("password must be unchanged after a failed reset")
	}
}

func TestResetPassword_RejectsBadPasswordWithoutBurningToken(t *testing.T) {
	user := existingUser(t, "dea@example.com", "oldpassword")
	env := newTestEnv(user)
	ctx := context.Background()
	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	raw := tokenFromMail(t, env.mail)

	for _, pw := range []string{"short", strings.Repeat("a", 73)} {
		if err := env.svc.ResetPassword(ctx, raw, pw); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("password %d chars: want ErrInvalidPassword, got %v", len(pw), err)
		}
	}
	if err := env.svc.ResetPassword(ctx, raw, "brandnewpass"); err != nil {
		t.Fatalf("token must still work after bad password attempts: %v", err)
	}
}

func TestResetPassword_GoogleOnlyAccountCanSetPassword(t *testing.T) {
	user := existingUser(t, "google@example.com", "") // tanpa password hash
	env := newTestEnv(user)
	ctx := context.Background()
	_ = env.svc.RequestPasswordReset(ctx, user.Email)
	raw := tokenFromMail(t, env.mail)

	if err := env.svc.ResetPassword(ctx, raw, "brandnewpass"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.svc.Login(ctx, LoginInput{Email: user.Email, Password: "brandnewpass"}); err != nil {
		t.Fatalf("password login must work after reset: %v", err)
	}
}

func TestResetEmail_EscapesHTML(t *testing.T) {
	msg := resetEmail(`<script>alert(1)</script>`, "a@b.c", "https://x.test/app/reset-password?token=a&b=c", time.Hour)
	if strings.Contains(msg.HTML, "<script>") {
		t.Fatal("name must be HTML-escaped")
	}
	if !strings.Contains(msg.HTML, "token=a&amp;b=c") {
		t.Fatal("link must be HTML-escaped inside href")
	}
	if !strings.Contains(msg.Text, "expires in 1 hour") {
		t.Fatalf("unexpected text: %s", msg.Text)
	}
}
