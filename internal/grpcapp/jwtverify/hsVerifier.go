package jwtverify

import (
	"context"
	"errors"
	"fmt"
	jwt "github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

// TokenType - ожидаемый тип токена в claims.
type TokenType string

const (
	TokenTypeAccess TokenType = "access"
)

// AccessClaims - claims, которые читает Workspace
// валидируем и достаeм user_id.
type AccessClaims struct {
	jwt.RegisteredClaims

	TokenType TokenType `json:"token_type"`
	// Закладка под сессии
	// TokenVersion int32  `json:"token_version,omitempty"`
	// SessionID    string `json:"session_id,omitempty"`
}

// HSOptions - параметры проверки HMAC JWT
type HSOptions struct {
	Issuer     string
	SigningKey []byte
	ClockSkew  time.Duration
}

// HSVerifier валидирует access JWT (HS256/384/512)
type HSVerifier struct {
	issuer string
	key    []byte
	skew   time.Duration
}

func NewHSVerifier(opt HSOptions) (*HSVerifier, error) {
	if strings.TrimSpace(opt.Issuer) == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	if len(opt.SigningKey) < 16 {
		return nil, fmt.Errorf("signing key is too short (min 16 bytes recommended)")
	}
	if opt.ClockSkew < 0 {
		return nil, fmt.Errorf("clock_skew must be >= 0")
	}

	return &HSVerifier{
		issuer: opt.Issuer,
		key:    append([]byte(nil), opt.SigningKey...),
		skew:   opt.ClockSkew,
	}, nil
}

// VerifyAccess валидирует access JWT и возвращает минимальные Claims
func (v *HSVerifier) VerifyAccess(ctx context.Context, token string) (Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Claims{}, ErrMissingToken
	}

	claims := new(AccessClaims)

	// Валидируем подпись и парсим claims
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			// ожидаем только HMAC
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %T", t.Method)
			}
			return v.key, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
			jwt.SigningMethodHS384.Alg(),
			jwt.SigningMethodHS512.Alg(),
		}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrExpiredToken
		}
		return Claims{}, ErrInvalidToken
	}
	if parsed == nil || !parsed.Valid {
		return Claims{}, ErrInvalidToken
	}

	if claims.Issuer != v.issuer {
		return Claims{}, ErrInvalidToken
	}

	if claims.TokenType != TokenTypeAccess {
		return Claims{}, ErrInvalidToken
	}

	userID := strings.TrimSpace(claims.Subject)
	if userID == "" {
		return Claims{}, ErrInvalidToken
	}

	now := time.Now()

	// exp: токен валиден пока now <= exp + skew
	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.IsZero() {
		return Claims{}, ErrInvalidToken
	}
	if now.After(claims.ExpiresAt.Time.Add(v.skew)) {
		return Claims{}, ErrExpiredToken
	}

	// nbf: токен валиден если now + skew >= nbf
	if claims.NotBefore != nil && !claims.NotBefore.Time.IsZero() {
		if now.Add(v.skew).Before(claims.NotBefore.Time) {
			return Claims{}, ErrInvalidToken
		}
	}

	return Claims{UserID: userID}, nil
}
