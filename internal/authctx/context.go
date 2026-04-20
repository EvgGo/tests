package authctx

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Уникальные ключи без коллизий между пакетами
type (
	userIDKey    struct{}
	requestIDKey struct{}
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserID(ctx context.Context) (string, bool) {
	v := ctx.Value(userIDKey{})
	s, ok := v.(string)
	return s, ok && s != ""
}

func MustUserID(ctx context.Context) string {
	if s, ok := UserID(ctx); ok {
		return s
	}
	return ""
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) (string, bool) {
	v := ctx.Value(requestIDKey{})
	s, ok := v.(string)
	return s, ok && s != ""
}

// GetUserIDFromContext  Получаем user_id из контекста
func GetUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := UserID(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "user ID not found in context")
	}
	return userID, nil
}
