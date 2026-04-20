package models

import "time"

type AssessmentStatus string

const (
	AssessmentStatusActive   AssessmentStatus = "active"
	AssessmentStatusArchived AssessmentStatus = "archived"
)

type AssessmentMode string

const (
	AssessmentModeSubtopic AssessmentMode = "subtopic"
	AssessmentModeGlobal   AssessmentMode = "global"
)

type AttemptStatus string

const (
	AttemptStatusInProgress AttemptStatus = "in_progress"
	AttemptStatusCompleted  AttemptStatus = "completed"
	AttemptStatusAbandoned  AttemptStatus = "abandoned"
	AttemptStatusExpired    AttemptStatus = "expired"
)

type FinishReason string

const (
	FinishReasonUnspecified FinishReason = "unspecified"
	FinishReasonUserFinish  FinishReason = "user_finish"
	FinishReasonUserAbandon FinishReason = "user_abandon"
	FinishReasonTimeout     FinishReason = "timeout"
	FinishReasonSystem      FinishReason = "system"
)

type FinishAction string

const (
	FinishActionComplete FinishAction = "complete"
	FinishActionAbandon  FinishAction = "abandon"
)

type Subject struct {
	ID        int64
	Code      string
	Title     string
	CreatedAt time.Time
}

type Subtopic struct {
	ID        int64
	SubjectID int64
	Code      string
	Title     string
	CreatedAt time.Time
}

type AssessmentSubtopicConfig struct {
	AssessmentID int64
	SubtopicID   int64

	Weight   float64
	Priority int

	MinItems        *int
	MaxItems        *int
	StartDifficulty *int
	StopConfidence  *float64
}

type EffectiveAssessmentSubtopic struct {
	AssessmentID int64
	SubtopicID   int64

	Weight   float64
	Priority int

	MinItems        int
	MaxItems        int
	StartDifficulty int
	StopConfidence  float64
}

type Assessment struct {
	ID        int64
	SubjectID int64

	Code        string
	Title       string
	Description string

	Status AssessmentStatus
	Mode   AssessmentMode

	MinItems        int
	MaxItems        int
	MinDifficulty   int
	MaxDifficulty   int
	StartDifficulty int
	StopConfidence  float64

	DurationSeconds int

	AssessmentSummary AttemptAssessmentSummary
	Subtopics         []AssessmentSubtopicConfig

	CreatedAt time.Time
	UpdatedAt time.Time
}

type AssessmentSummary struct {
	ID        int64
	SubjectID int64

	Code        string
	Title       string
	Description string

	Status AssessmentStatus
	Mode   AssessmentMode

	DurationSeconds int
}

type QuestionOption struct {
	ID         int64
	QuestionID int64
	Body       string
	Position   int
}

type Question struct {
	ID         int64
	SubtopicID int64

	Difficulty int
	Body       string

	CorrectOptionID *int64
	Options         []QuestionOption
}

type AttemptSubtopicState struct {
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

	Weight   float64
	Priority int

	MinItems        int
	MaxItems        int
	StartDifficulty int
	StopConfidence  float64

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Attempt struct {
	ID           int64
	AssessmentID int64
	UserID       string

	Status       AttemptStatus
	FinishReason FinishReason

	SequenceNo int

	StartedAt   time.Time
	ExpiresAt   time.Time
	CompletedAt *time.Time

	DurationSeconds int

	OverallLevel      int
	OverallLevelScore float64
	OverallConfidence float64

	SubtopicStates    []AttemptSubtopicState
	AssessmentSummary AttemptAssessmentSummary

	CreatedAt time.Time
	UpdatedAt time.Time
}

type AttemptProgress struct {
	AttemptID int64
	Status    AttemptStatus

	SequenceNo int

	StartedAt time.Time
	ExpiresAt time.Time

	RemainingSeconds int64

	OverallLevel      int
	OverallLevelScore float64
	OverallConfidence float64
}

type AttemptAnswer struct {
	AttemptID int64
	SeqNo     int

	QuestionID int64
	SubtopicID int64
	Difficulty int

	SelectedOptionID int64
	IsCorrect        bool

	AnsweredAt time.Time
}

type ListSubjectsFilter struct {
	Query     string
	PageSize  int
	PageToken string
}

type ListSubtopicsFilter struct {
	SubjectID int64
	Query     string
	PageSize  int
	PageToken string
}

type ListAssessmentsFilter struct {
	SubjectID int64
	Query     string
	Status    AssessmentStatus
	PageSize  int
	PageToken string
}

type ListMyAttemptsFilter struct {
	UserID       string
	AssessmentID int64
	Status       AttemptStatus
	PageSize     int
	PageToken    string
}

type AttemptAssessmentSummary struct {
	AssessmentID    int64
	AssessmentCode  string
	AssessmentTitle string

	SubjectID    int64
	SubjectCode  string
	SubjectTitle string

	Mode AssessmentMode
}
