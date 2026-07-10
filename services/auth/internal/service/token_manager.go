package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type UserClaims struct {
	UserID	string `json:"user_id"`
	Email	string `json:"email"`
	jwt.RegisteredClaims
}

type TokenManager interface {
	GenerateAccessToken(userID uuid.UUID, email string)(string,error)
	GenerateRefreshToken()(string, error)
	ValidateAccessToken(tokenstr string)(*UserClaims, error)
}

type tokenManager struct {
	secretKey		[]byte
	accessDuration	time.Duration
}

func NewTokenManager(secretKey string, accessDuration time.Duration) TokenManager {
	return &tokenManager{
		secretKey:      []byte(secretKey),
		accessDuration: accessDuration,
	}
}

func (m *tokenManager) GenerateAccessToken(userID uuid.UUID, email string)(string, error){
	claims := UserClaims{
		UserID:	userID.String(),
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessDuration)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(m.secretKey)
}

func (m *tokenManager) GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (m *tokenManager) ValidateAccessToken(tokenStr string)(*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("Invalid token claims")
	}

	return claims, nil
}