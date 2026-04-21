package repo

import (
	"context"
	"testing/internal/models"
)

type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type CatalogRepo interface {
	ListSubjects(ctx context.Context, filter models.ListSubjectsFilter) ([]models.Subject, string, error)
	ListSubtopics(ctx context.Context, filter models.ListSubtopicsFilter) ([]models.Subtopic, string, error)
	ListAssessments(ctx context.Context, filter models.ListAssessmentsFilter) ([]models.AssessmentSummary, string, error)

	GetAssessment(ctx context.Context, assessmentID int64, includeSubtopics bool) (*models.Assessment, error)
	GetAssessmentForStart(ctx context.Context, assessmentID int64) (*models.Assessment, error)
}

type QuestionRepo interface {
	GetQuestionForAnswer(ctx context.Context, questionID int64) (*models.Question, error)
	GetQuestionWithOptions(ctx context.Context, questionID int64) (*models.Question, error)

	OptionBelongsToQuestion(ctx context.Context, questionID int64, optionID int64) (bool, error)

	FindNextQuestionByDifficulties(ctx context.Context, attemptID int64, subtopicID int64,
		difficulties []int) (*models.Question, error)

	FindNextGlobalQuestionByDifficulties(ctx context.Context, attemptID int64, subjectID int64,
		difficulties []int) (*models.Question, error)
}

type AttemptRepo interface {
	FindInProgressAttempt(ctx context.Context, userID string, assessmentID int64) (*models.Attempt, error)

	CreateAttempt(ctx context.Context, input models.CreateAttemptInput) (*models.Attempt, error)
	CreateAttemptState(ctx context.Context, input models.CreateAttemptStateInput) error

	GetAttempt(ctx context.Context, attemptID int64, userID string, includeSubtopicStates bool) (*models.Attempt, error)
	GetAttemptForUpdate(ctx context.Context, attemptID int64, userID string) (*models.Attempt, error)

	ListAttemptStates(ctx context.Context, attemptID int64) ([]models.AttemptSubtopicState, error)
	ListAttemptStatesForUpdate(ctx context.Context, attemptID int64) ([]models.AttemptSubtopicState, error)

	UpdateAttemptState(ctx context.Context, input models.UpdateAttemptStateInput) error

	CreateAttemptAnswer(ctx context.Context, input models.CreateAttemptAnswerInput) error
	UpdateAttemptAggregate(ctx context.Context, input models.UpdateAttemptAggregateInput) error

	FinishAttempt(ctx context.Context, input models.FinishAttemptInput) (*models.Attempt, error)

	ListMyAttempts(ctx context.Context, filter models.ListMyAttemptsFilter) ([]models.Attempt, string, error)
}
