package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

// GoogleOAuth menjalankan authorization code flow di sisi Gateway:
// Gateway menukar `code` menjadi `id_token`, lalu Auth Service yang memverifikasi id_token tsb
// (rpc GoogleAuth). Client secret tidak pernah keluar dari Gateway.
type GoogleOAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	HTTPClient   *http.Client
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *GoogleOAuth) Enabled() bool {
	return g != nil && g.ClientID != "" && g.ClientSecret != ""
}

func (g *GoogleOAuth) AuthCodeURL(state string) string {
	q := url.Values{
		"client_id":     {g.ClientID},
		"redirect_uri":  {g.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"prompt":        {"select_account"},
	}
	return googleAuthURL + "?" + q.Encode()
}

// ExchangeIDToken menukar authorization code menjadi Google ID token.
func (g *GoogleOAuth) ExchangeIDToken(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"redirect_uri":  {g.RedirectURL},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := g.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("google token exchange failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read google token response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token exchange returned %d: %s", res.StatusCode, body)
	}

	var payload struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("invalid google token response: %w", err)
	}
	if payload.IDToken == "" {
		return "", fmt.Errorf("google token response has no id_token")
	}
	return payload.IDToken, nil
}
