package models

import "time"

type CreateAttemptInput struct {
	AssessmentID    int64
	UserID          string
	StartedAt       time.Time
	ExpiresAt       time.Time
	DurationSeconds int
}

type CreateAttemptStateInput struct {
	AttemptID  int64
	SubtopicID int64

	LevelScore     float64
	EstimatedLevel int
	Confidence     float64

	Weight   float64
	Priority int

	MinItems        int
	MaxItems        int
	StartDifficulty int
	StopConfidence  float64
}

type UpdateAttemptStateInput struct {
	AttemptID  int64
	SubtopicID int64

	AskedCount   int
	CorrectCount int
	WrongCount   int

	ConsecutiveCorrect int
	ConsecutiveWrong   int

	LevelScore     float64
	EstimatedLevel int
	Confidence     float64

	IsLocked bool

	LastQuestionID    *int64
	LastAnswerCorrect *bool
}

type CreateAttemptAnswerInput struct {
	AttemptID int64
	SeqNo     int

	QuestionID int64
	SubtopicID int64
	Difficulty int

	SelectedOptionID int64
	IsCorrect        bool
}

type UpdateAttemptAggregateInput struct {
	AttemptID int64

	SequenceNo int

	OverallLevel      int
	OverallLevelScore float64
	OverallConfidence float64
}

type FinishAttemptInput struct {
	AttemptID int64

	Status       AttemptStatus
	FinishReason FinishReason
	CompletedAt  time.Time

	OverallLevel      int
	OverallLevelScore float64
	OverallConfidence float64
}
