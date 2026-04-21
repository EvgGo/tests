package assessmentsvc

import (
	"strings"
	"testing/internal/models"
	"testing/internal/repo"
	"time"
)

type Service struct {
	tx        repo.TxManager
	catalog   repo.CatalogRepo
	questions repo.QuestionRepo
	attempts  repo.AttemptRepo

	clock func() time.Time
}

func New(
	tx repo.TxManager,
	catalog repo.CatalogRepo,
	questions repo.QuestionRepo,
	attempts repo.AttemptRepo,
) *Service {
	return &Service{
		tx:        tx,
		catalog:   catalog,
		questions: questions,
		attempts:  attempts,
		clock:     time.Now,
	}
}

func (s *Service) SetClock(clock func() time.Time) {
	if clock != nil {
		s.clock = clock
	}
}

type StartAttemptInput struct {
	UserID              string
	AssessmentID        int64
	RestartIfInProgress bool
}

type StartAttemptResult struct {
	Attempt       models.Attempt
	FirstQuestion *models.Question
	Progress      models.AttemptProgress
}

type GetNextQuestionInput struct {
	UserID    string
	AttemptID int64
}

type GetNextQuestionResult struct {
	Completed    bool
	Progress     models.AttemptProgress
	NextQuestion *models.Question
}

type SubmitAnswerInput struct {
	UserID           string
	AttemptID        int64
	QuestionID       int64
	SelectedOptionID int64
}

type SubmitAnswerResult struct {
	Completed    bool
	Progress     models.AttemptProgress
	NextQuestion *models.Question
}

type FinishAttemptInput struct {
	UserID    string
	AttemptID int64
	Action    models.FinishAction
}

func normalizeUserID(userID string) string {
	return strings.TrimSpace(userID)
}

func isTerminalAttemptStatus(status models.AttemptStatus) bool {
	return status == models.AttemptStatusCompleted ||
		status == models.AttemptStatusAbandoned ||
		status == models.AttemptStatusExpired
}

func isAttemptExpired(attempt models.Attempt, now time.Time) bool {
	return attempt.Status == models.AttemptStatusInProgress && !now.Before(attempt.ExpiresAt)
}

func buildProgress(attempt models.Attempt, now time.Time) models.AttemptProgress {
	remainingSeconds := int64(0)

	if attempt.Status == models.AttemptStatusInProgress && now.Before(attempt.ExpiresAt) {
		remainingSeconds = int64(attempt.ExpiresAt.Sub(now).Seconds())
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
	}

	return models.AttemptProgress{
		AttemptID:         attempt.ID,
		Status:            attempt.Status,
		SequenceNo:        attempt.SequenceNo,
		StartedAt:         attempt.StartedAt,
		ExpiresAt:         attempt.ExpiresAt,
		RemainingSeconds:  remainingSeconds,
		OverallLevel:      attempt.OverallLevel,
		OverallLevelScore: attempt.OverallLevelScore,
		OverallConfidence: attempt.OverallConfidence,
	}
}

func applyOverallToAttempt(attempt *models.Attempt, overall OverallResult) {
	attempt.OverallLevel = overall.OverallLevel
	attempt.OverallLevelScore = overall.OverallLevelScore
	attempt.OverallConfidence = overall.OverallConfidence
}

func stateToUpdateInput(state models.AttemptSubtopicState) models.UpdateAttemptStateInput {
	return models.UpdateAttemptStateInput{
		AttemptID:          state.AttemptID,
		SubtopicID:         state.SubtopicID,
		AskedCount:         state.AskedCount,
		CorrectCount:       state.CorrectCount,
		WrongCount:         state.WrongCount,
		ConsecutiveCorrect: state.ConsecutiveCorrect,
		ConsecutiveWrong:   state.ConsecutiveWrong,
		LevelScore:         state.LevelScore,
		EstimatedLevel:     state.EstimatedLevel,
		Confidence:         state.Confidence,
		IsLocked:           state.IsLocked,
		LastQuestionID:     state.LastQuestionID,
		LastAnswerCorrect:  state.LastAnswerCorrect,
	}
}

func replaceState(
	states []models.AttemptSubtopicState,
	nextState models.AttemptSubtopicState,
) []models.AttemptSubtopicState {
	for i := range states {
		if states[i].SubtopicID == nextState.SubtopicID {
			states[i] = nextState
			return states
		}
	}

	return states
}
