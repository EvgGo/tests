package grpc

import (
	"context"
	"strings"
	"testing/internal/authctx"
	"testing/internal/repo"
)

func userIDFromContext(ctx context.Context) (string, error) {

	userID, ok := authctx.UserID(ctx)
	if !ok {
		return "", repo.ErrForbidden
	}

	return strings.TrimSpace(userID), nil
}
