package mappers

import (
	"testing/internal/models"
	"time"

	testingv1 "github.com/EvgGo/proto/proto/gen/go/tests"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func SubjectToProto(subject models.Subject) *testingv1.Subject {
	return &testingv1.Subject{
		Id:        subject.ID,
		Code:      subject.Code,
		Title:     subject.Title,
		CreatedAt: TimeToProto(subject.CreatedAt),
	}
}

func SubtopicToProto(subtopic models.Subtopic) *testingv1.Subtopic {
	return &testingv1.Subtopic{
		Id:        subtopic.ID,
		SubjectId: subtopic.SubjectID,
		Code:      subtopic.Code,
		Title:     subtopic.Title,
		CreatedAt: TimeToProto(subtopic.CreatedAt),
	}
}

func AssessmentSummaryToProto(assessment models.AssessmentSummary) *testingv1.AssessmentSummary {
	return &testingv1.AssessmentSummary{
		Id:              assessment.ID,
		SubjectId:       assessment.SubjectID,
		Code:            assessment.Code,
		Title:           assessment.Title,
		Description:     assessment.Description,
		Status:          AssessmentStatusToProto(assessment.Status),
		Mode:            AssessmentModeToProto(assessment.Mode),
		DurationSeconds: int32(assessment.DurationSeconds),
	}
}

func AssessmentToProto(assessment models.Assessment) *testingv1.Assessment {
	subtopics := make([]*testingv1.AssessmentSubtopicConfig, 0, len(assessment.Subtopics))

	for _, subtopic := range assessment.Subtopics {
		subtopics = append(subtopics, assessmentSubtopicConfigToProto(subtopic))
	}

	return &testingv1.Assessment{
		Id:              assessment.ID,
		SubjectId:       assessment.SubjectID,
		Code:            assessment.Code,
		Title:           assessment.Title,
		Description:     assessment.Description,
		Status:          AssessmentStatusToProto(assessment.Status),
		MinItems:        int32(assessment.MinItems),
		MaxItems:        int32(assessment.MaxItems),
		MinDifficulty:   int32(assessment.MinDifficulty),
		MaxDifficulty:   int32(assessment.MaxDifficulty),
		StartDifficulty: int32(assessment.StartDifficulty),
		StopConfidence:  assessment.StopConfidence,
		DurationSeconds: int32(assessment.DurationSeconds),
		Mode:            AssessmentModeToProto(assessment.Mode),
		CreatedAt:       TimeToProto(assessment.CreatedAt),
		UpdatedAt:       TimeToProto(assessment.UpdatedAt),
		Subtopics:       subtopics,
	}
}

func assessmentSubtopicConfigToProto(config models.AssessmentSubtopicConfig) *testingv1.AssessmentSubtopicConfig {
	result := &testingv1.AssessmentSubtopicConfig{
		AssessmentId: config.AssessmentID,
		SubtopicId:   config.SubtopicID,
		Weight:       config.Weight,
		Priority:     int32(config.Priority),
	}

	if config.MinItems != nil {
		value := int32(*config.MinItems)
		result.MinItems = &value
	}
	if config.MaxItems != nil {
		value := int32(*config.MaxItems)
		result.MaxItems = &value
	}
	if config.StartDifficulty != nil {
		value := int32(*config.StartDifficulty)
		result.StartDifficulty = &value
	}
	if config.StopConfidence != nil {
		result.StopConfidence = config.StopConfidence
	}

	return result
}

func QuestionToProto(question models.Question) *testingv1.Question {
	options := make([]*testingv1.QuestionOption, 0, len(question.Options))

	for _, option := range question.Options {
		options = append(options, QuestionOptionToProto(option))
	}

	return &testingv1.Question{
		Id:         question.ID,
		SubtopicId: question.SubtopicID,
		Difficulty: int32(question.Difficulty),
		Body:       question.Body,
		Options:    options,
	}
}

func QuestionOptionToProto(option models.QuestionOption) *testingv1.QuestionOption {
	return &testingv1.QuestionOption{
		Id:         option.ID,
		QuestionId: option.QuestionID,
		Body:       option.Body,
		Position:   int32(option.Position),
	}
}

func AttemptToProto(attempt models.Attempt) *testingv1.Attempt {
	states := make([]*testingv1.AttemptSubtopicState, 0, len(attempt.SubtopicStates))

	for _, state := range attempt.SubtopicStates {
		states = append(states, AttemptSubtopicStateToProto(state))
	}

	return &testingv1.Attempt{
		Id:                attempt.ID,
		AssessmentId:      attempt.AssessmentID,
		Status:            AttemptStatusToProto(attempt.Status),
		FinishReason:      FinishReasonToProto(attempt.FinishReason),
		SequenceNo:        int32(attempt.SequenceNo),
		StartedAt:         TimeToProto(attempt.StartedAt),
		ExpiresAt:         TimeToProto(attempt.ExpiresAt),
		CompletedAt:       OptionalTimeToProto(attempt.CompletedAt),
		DurationSeconds:   int32(attempt.DurationSeconds),
		OverallLevel:      int32(attempt.OverallLevel),
		OverallLevelScore: attempt.OverallLevelScore,
		OverallConfidence: attempt.OverallConfidence,
		SubtopicStates:    states,
		AssessmentSummary: AttemptAssessmentSummaryToProto(attempt.AssessmentSummary),
	}
}

func AttemptProgressToProto(progress models.AttemptProgress) *testingv1.AttemptProgress {
	return &testingv1.AttemptProgress{
		AttemptId:         progress.AttemptID,
		Status:            AttemptStatusToProto(progress.Status),
		SequenceNo:        int32(progress.SequenceNo),
		StartedAt:         TimeToProto(progress.StartedAt),
		ExpiresAt:         TimeToProto(progress.ExpiresAt),
		RemainingSeconds:  progress.RemainingSeconds,
		OverallLevel:      int32(progress.OverallLevel),
		OverallLevelScore: progress.OverallLevelScore,
		OverallConfidence: progress.OverallConfidence,
	}
}

func AttemptSubtopicStateToProto(state models.AttemptSubtopicState) *testingv1.AttemptSubtopicState {
	result := &testingv1.AttemptSubtopicState{
		AttemptId:          state.AttemptID,
		SubtopicId:         state.SubtopicID,
		AskedCount:         int32(state.AskedCount),
		CorrectCount:       int32(state.CorrectCount),
		WrongCount:         int32(state.WrongCount),
		ConsecutiveCorrect: int32(state.ConsecutiveCorrect),
		ConsecutiveWrong:   int32(state.ConsecutiveWrong),
		LevelScore:         state.LevelScore,
		EstimatedLevel:     int32(state.EstimatedLevel),
		Confidence:         state.Confidence,
		IsLocked:           state.IsLocked,
		Weight:             state.Weight,
		Priority:           int32(state.Priority),
		MinItems:           int32(state.MinItems),
		MaxItems:           int32(state.MaxItems),
		StartDifficulty:    int32(state.StartDifficulty),
		StopConfidence:     state.StopConfidence,
	}

	if state.LastQuestionID != nil {
		result.LastQuestionId = state.LastQuestionID
	}
	if state.LastAnswerCorrect != nil {
		result.LastAnswerCorrect = state.LastAnswerCorrect
	}

	return result
}

func AssessmentStatusToProto(status models.AssessmentStatus) testingv1.AssessmentStatus {
	switch status {
	case models.AssessmentStatusActive:
		return testingv1.AssessmentStatus_ASSESSMENT_STATUS_ACTIVE
	case models.AssessmentStatusArchived:
		return testingv1.AssessmentStatus_ASSESSMENT_STATUS_ARCHIVED
	default:
		return testingv1.AssessmentStatus_ASSESSMENT_STATUS_UNSPECIFIED
	}
}

func AssessmentStatusFromProto(status testingv1.AssessmentStatus) models.AssessmentStatus {
	switch status {
	case testingv1.AssessmentStatus_ASSESSMENT_STATUS_ACTIVE:
		return models.AssessmentStatusActive
	case testingv1.AssessmentStatus_ASSESSMENT_STATUS_ARCHIVED:
		return models.AssessmentStatusArchived
	default:
		return ""
	}
}

func AttemptStatusToProto(status models.AttemptStatus) testingv1.AttemptStatus {
	switch status {
	case models.AttemptStatusInProgress:
		return testingv1.AttemptStatus_ATTEMPT_STATUS_IN_PROGRESS
	case models.AttemptStatusCompleted:
		return testingv1.AttemptStatus_ATTEMPT_STATUS_COMPLETED
	case models.AttemptStatusAbandoned:
		return testingv1.AttemptStatus_ATTEMPT_STATUS_ABANDONED
	case models.AttemptStatusExpired:
		return testingv1.AttemptStatus_ATTEMPT_STATUS_EXPIRED
	default:
		return testingv1.AttemptStatus_ATTEMPT_STATUS_UNSPECIFIED
	}
}

func AttemptStatusFromProto(status testingv1.AttemptStatus) models.AttemptStatus {
	switch status {
	case testingv1.AttemptStatus_ATTEMPT_STATUS_IN_PROGRESS:
		return models.AttemptStatusInProgress
	case testingv1.AttemptStatus_ATTEMPT_STATUS_COMPLETED:
		return models.AttemptStatusCompleted
	case testingv1.AttemptStatus_ATTEMPT_STATUS_ABANDONED:
		return models.AttemptStatusAbandoned
	case testingv1.AttemptStatus_ATTEMPT_STATUS_EXPIRED:
		return models.AttemptStatusExpired
	default:
		return ""
	}
}

func FinishReasonToProto(reason models.FinishReason) testingv1.FinishReason {
	switch reason {
	case models.FinishReasonUserFinish:
		return testingv1.FinishReason_FINISH_REASON_USER_FINISH
	case models.FinishReasonUserAbandon:
		return testingv1.FinishReason_FINISH_REASON_USER_ABANDON
	case models.FinishReasonTimeout:
		return testingv1.FinishReason_FINISH_REASON_TIMEOUT
	case models.FinishReasonSystem:
		return testingv1.FinishReason_FINISH_REASON_SYSTEM
	default:
		return testingv1.FinishReason_FINISH_REASON_UNSPECIFIED
	}
}

func FinishActionFromProto(action testingv1.FinishAction) models.FinishAction {
	switch action {
	case testingv1.FinishAction_FINISH_ACTION_COMPLETE:
		return models.FinishActionComplete
	case testingv1.FinishAction_FINISH_ACTION_ABANDON:
		return models.FinishActionAbandon
	default:
		return ""
	}
}

func TimeToProto(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}

	return timestamppb.New(value)
}

func OptionalTimeToProto(value *time.Time) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}

	return timestamppb.New(*value)
}

func AssessmentModeToProto(mode models.AssessmentMode) testingv1.AssessmentMode {
	switch mode {
	case models.AssessmentModeSubtopic:
		return testingv1.AssessmentMode_ASSESSMENT_MODE_SUBTOPIC
	case models.AssessmentModeGlobal:
		return testingv1.AssessmentMode_ASSESSMENT_MODE_GLOBAL
	default:
		return testingv1.AssessmentMode_ASSESSMENT_MODE_UNSPECIFIED
	}
}

func AssessmentModeFromProto(mode testingv1.AssessmentMode) models.AssessmentMode {
	switch mode {
	case testingv1.AssessmentMode_ASSESSMENT_MODE_SUBTOPIC:
		return models.AssessmentModeSubtopic
	case testingv1.AssessmentMode_ASSESSMENT_MODE_GLOBAL:
		return models.AssessmentModeGlobal
	default:
		return ""
	}
}

func AttemptAssessmentSummaryToProto(summary models.AttemptAssessmentSummary) *testingv1.AttemptAssessmentSummary {
	return &testingv1.AttemptAssessmentSummary{
		AssessmentId:    summary.AssessmentID,
		AssessmentCode:  summary.AssessmentCode,
		AssessmentTitle: summary.AssessmentTitle,
		SubjectId:       summary.SubjectID,
		SubjectCode:     summary.SubjectCode,
		SubjectTitle:    summary.SubjectTitle,
		Mode:            AssessmentModeToProto(summary.Mode),
	}
}
