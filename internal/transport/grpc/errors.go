package grpc

import (
	"errors"
	"testing/internal/repo"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func toStatusErr(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, repo.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, repo.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, repo.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, repo.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())

	case errors.Is(err, repo.ErrExpired):
		return status.Error(codes.DeadlineExceeded, err.Error())

	case errors.Is(err, repo.ErrInvalidState):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, repo.ErrOptionMismatch):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, repo.ErrQuestionMismatch):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, repo.ErrNoQuestions):
		return status.Error(codes.FailedPrecondition, err.Error())

	default:
		return status.Error(codes.Internal, "internal error")
	}
}
