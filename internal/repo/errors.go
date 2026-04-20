package repo

import "errors"

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrAlreadyExists    = errors.New("already exists")
	ErrExpired          = errors.New("expired")
	ErrInvalidState     = errors.New("invalid state")
	ErrNoQuestions      = errors.New("no questions")
	ErrOptionMismatch   = errors.New("selected option does not belong to question")
	ErrQuestionMismatch = errors.New("question does not match current attempt state")
)
