package jwtverify

import (
	"context"
	"errors"
)

type Claims struct {
	UserID string
}

// Verifier - интерфейс проверки access JWT
type Verifier interface {
	VerifyAccess(ctx context.Context, token string) (Claims, error)
}

var (
	ErrMissingToken = errors.New("missing token")
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)
