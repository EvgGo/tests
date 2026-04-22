package grpc

import (
	"context"
	"log/slog"

	assessmentv1 "github.com/EvgGo/proto/proto/gen/go/tests"

	"testing/internal/models"
	assessmentsvc "testing/internal/service/assessment"
	"testing/internal/transport/grpc/mappers"
)

type assessmentService interface {
	ListSubjects(ctx context.Context, filter models.ListSubjectsFilter) ([]models.Subject, string, error)
	ListSubtopics(ctx context.Context, filter models.ListSubtopicsFilter) ([]models.Subtopic, string, error)
	ListAssessments(ctx context.Context, filter models.ListAssessmentsFilter) ([]models.AssessmentSummary, string, error)
	GetAssessment(ctx context.Context, assessmentID int64, includeSubtopics bool) (*models.Assessment, error)

	StartAttempt(ctx context.Context, input assessmentsvc.StartAttemptInput) (*assessmentsvc.StartAttemptResult, error)
	GetAttempt(ctx context.Context, userID string, attemptID int64, includeSubtopicStates bool) (*models.Attempt, error)
	GetAttemptProgress(ctx context.Context, userID string, attemptID int64) (*models.AttemptProgress, error)
	GetNextQuestion(ctx context.Context, input assessmentsvc.GetNextQuestionInput) (*assessmentsvc.GetNextQuestionResult, error)
	SubmitAnswer(ctx context.Context, input assessmentsvc.SubmitAnswerInput) (*assessmentsvc.SubmitAnswerResult, error)
	FinishAttempt(ctx context.Context, input assessmentsvc.FinishAttemptInput) (*models.Attempt, error)
	ListMyAttempts(ctx context.Context, filter models.ListMyAttemptsFilter) ([]models.Attempt, string, error)
}

type Server struct {
	assessmentv1.UnimplementedAdaptiveTestingServer

	log     *slog.Logger
	service assessmentService
}

func NewServer(log *slog.Logger, service assessmentService) *Server {
	if log == nil {
		log = slog.Default()
	}

	return &Server{
		log:     log,
		service: service,
	}
}

func (s *Server) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

func (s *Server) ListSubjects(
	ctx context.Context,
	req *assessmentv1.ListSubjectsRequest,
) (*assessmentv1.ListSubjectsResponse, error) {

	log := s.logger().With(
		"method", "ListSubjects",
		"query", req.GetQuery(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	log.Info("grpc request started")

	subjects, nextPageToken, err := s.service.ListSubjects(ctx, models.ListSubjectsFilter{
		Query:     req.GetQuery(),
		PageSize:  int(req.GetPageSize()),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		log.Error("service.ListSubjects failed", "err", err)
		return nil, toStatusErr(err)
	}

	items := make([]*assessmentv1.Subject, 0, len(subjects))
	for _, subject := range subjects {
		items = append(items, mappers.SubjectToProto(subject))
	}

	log.Info("grpc request completed",
		"items_count", len(items),
		"next_page_token", nextPageToken,
	)

	return &assessmentv1.ListSubjectsResponse{
		Subjects:      items,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Server) ListSubtopics(
	ctx context.Context,
	req *assessmentv1.ListSubtopicsRequest,
) (*assessmentv1.ListSubtopicsResponse, error) {

	log := s.logger().With(
		"method", "ListSubtopics",
		"subject_id", req.GetSubjectId(),
		"query", req.GetQuery(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	log.Info("grpc request started")

	subtopics, nextPageToken, err := s.service.ListSubtopics(ctx, models.ListSubtopicsFilter{
		SubjectID: req.GetSubjectId(),
		Query:     req.GetQuery(),
		PageSize:  int(req.GetPageSize()),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		log.Error("service.ListSubtopics failed", "err", err)
		return nil, toStatusErr(err)
	}

	items := make([]*assessmentv1.Subtopic, 0, len(subtopics))
	for _, subtopic := range subtopics {
		items = append(items, mappers.SubtopicToProto(subtopic))
	}

	log.Info("grpc request completed",
		"items_count", len(items),
		"next_page_token", nextPageToken,
	)

	return &assessmentv1.ListSubtopicsResponse{
		Subtopics:     items,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Server) ListAssessments(
	ctx context.Context,
	req *assessmentv1.ListAssessmentsRequest,
) (*assessmentv1.ListAssessmentsResponse, error) {

	log := s.logger().With(
		"method", "ListAssessments",
		"subject_id", req.GetSubjectId(),
		"query", req.GetQuery(),
		"status", req.GetStatus().String(),
		"mode", req.GetMode().String(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	log.Info("grpc request started")

	assessments, nextPageToken, err := s.service.ListAssessments(ctx, models.ListAssessmentsFilter{
		SubjectID: req.GetSubjectId(),
		Query:     req.GetQuery(),
		Status:    mappers.AssessmentStatusFromProto(req.GetStatus()),
		Mode:      mappers.AssessmentModeFromProto(req.GetMode()),
		PageSize:  int(req.GetPageSize()),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		log.Error("service.ListAssessments failed", "err", err)
		return nil, toStatusErr(err)
	}

	items := make([]*assessmentv1.AssessmentSummary, 0, len(assessments))
	for _, assessm := range assessments {
		items = append(items, mappers.AssessmentSummaryToProto(assessm))
	}

	globalCount := 0
	subtopicCount := 0

	for _, assessm := range assessments {
		switch assessm.Mode {
		case models.AssessmentModeGlobal:
			globalCount++
		case models.AssessmentModeSubtopic:
			subtopicCount++
		}
	}

	log.Info("grpc request completed",
		"items_count", len(items),
		"global_count", globalCount,
		"subtopic_count", subtopicCount,
		"next_page_token", nextPageToken,
	)

	return &assessmentv1.ListAssessmentsResponse{
		Assessments:   items,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *Server) GetAssessment(
	ctx context.Context,
	req *assessmentv1.GetAssessmentRequest,
) (*assessmentv1.Assessment, error) {

	log := s.logger().With(
		"method", "GetAssessment",
		"assessment_id", req.GetAssessmentId(),
		"include_subtopics", req.GetIncludeSubtopics(),
	)

	log.Info("grpc request started")

	assessm, err := s.service.GetAssessment(ctx, req.GetAssessmentId(), req.GetIncludeSubtopics())
	if err != nil {
		log.Error("service.GetAssessment failed",
			"assessment_id", req.GetAssessmentId(),
			"err", err,
		)
		return nil, toStatusErr(err)
	}

	log.Info("grpc request completed",
		"assessment_id", assessm.ID,
		"title", assessm.Title,
		"assessment_mode", assessm.Mode,
		"subtopics_count", len(assessm.Subtopics),
	)

	return mappers.AssessmentToProto(*assessm), nil
}

func (s *Server) StartAttempt(
	ctx context.Context,
	req *assessmentv1.StartAttemptRequest,
) (*assessmentv1.StartAttemptResponse, error) {
	log := s.logger().With(
		"method", "StartAttempt",
		"assessment_id", req.GetAssessmentId(),
		"restart_if_in_progress", req.GetRestartIfInProgress(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	result, err := s.service.StartAttempt(ctx, assessmentsvc.StartAttemptInput{
		UserID:              userID,
		AssessmentID:        req.GetAssessmentId(),
		RestartIfInProgress: req.GetRestartIfInProgress(),
	})
	if err != nil {
		log.Error("service.StartAttempt failed", "err", err)
		return nil, toStatusErr(err)
	}

	resp := &assessmentv1.StartAttemptResponse{
		Attempt:  mappers.AttemptToProto(result.Attempt),
		Progress: mappers.AttemptProgressToProto(result.Progress),
	}

	if result.FirstQuestion != nil {
		resp.FirstQuestion = mappers.QuestionToProto(*result.FirstQuestion)
	}

	log.Info("grpc request completed",
		"attempt_id", result.Attempt.ID,
		"assessment_id", result.Attempt.AssessmentSummary.AssessmentID,
		"assessment_code", result.Attempt.AssessmentSummary.AssessmentCode,
		"assessment_title", result.Attempt.AssessmentSummary.AssessmentTitle,
		"subject_id", result.Attempt.AssessmentSummary.SubjectID,
		"subject_code", result.Attempt.AssessmentSummary.SubjectCode,
		"subject_title", result.Attempt.AssessmentSummary.SubjectTitle,
		"assessment_mode", result.Attempt.AssessmentSummary.Mode,
		"status", result.Attempt.Status,
		"has_first_question", result.FirstQuestion != nil,
		"remaining_seconds", result.Progress.RemainingSeconds,
	)

	return resp, nil
}

func (s *Server) GetAttempt(
	ctx context.Context,
	req *assessmentv1.GetAttemptRequest,
) (*assessmentv1.Attempt, error) {
	log := s.logger().With(
		"method", "GetAttempt",
		"attempt_id", req.GetAttemptId(),
		"include_subtopic_states", req.GetIncludeSubtopicStates(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	attempt, err := s.service.GetAttempt(
		ctx,
		userID,
		req.GetAttemptId(),
		req.GetIncludeSubtopicStates(),
	)
	if err != nil {
		log.Error("service.GetAttempt failed", "err", err)
		return nil, toStatusErr(err)
	}

	log.Info("grpc request completed",
		"attempt_id", attempt.ID,
		"assessment_id", attempt.AssessmentSummary.AssessmentID,
		"assessment_code", attempt.AssessmentSummary.AssessmentCode,
		"assessment_title", attempt.AssessmentSummary.AssessmentTitle,
		"subject_id", attempt.AssessmentSummary.SubjectID,
		"subject_code", attempt.AssessmentSummary.SubjectCode,
		"subject_title", attempt.AssessmentSummary.SubjectTitle,
		"assessment_mode", attempt.AssessmentSummary.Mode,
		"status", attempt.Status,
		"sequence_no", attempt.SequenceNo,
		"subtopic_states_count", len(attempt.SubtopicStates),
	)

	return mappers.AttemptToProto(*attempt), nil
}

func (s *Server) GetAttemptProgress(
	ctx context.Context,
	req *assessmentv1.GetAttemptProgressRequest,
) (*assessmentv1.AttemptProgress, error) {

	log := s.logger().With(
		"method", "GetAttemptProgress",
		"attempt_id", req.GetAttemptId(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	progress, err := s.service.GetAttemptProgress(ctx, userID, req.GetAttemptId())
	if err != nil {
		log.Error("service.GetAttemptProgress failed", "err", err)
		return nil, toStatusErr(err)
	}

	log.Info("grpc request completed",
		"attempt_id", progress.AttemptID,
		"status", progress.Status,
		"remaining_seconds", progress.RemainingSeconds,
		"overall_level", progress.OverallLevel,
		"overall_confidence", progress.OverallConfidence,
	)

	return mappers.AttemptProgressToProto(*progress), nil
}

func (s *Server) GetNextQuestion(
	ctx context.Context,
	req *assessmentv1.GetNextQuestionRequest,
) (*assessmentv1.GetNextQuestionResponse, error) {

	log := s.logger().With(
		"method", "GetNextQuestion",
		"attempt_id", req.GetAttemptId(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	result, err := s.service.GetNextQuestion(ctx, assessmentsvc.GetNextQuestionInput{
		UserID:    userID,
		AttemptID: req.GetAttemptId(),
	})
	if err != nil {
		log.Error("service.GetNextQuestion failed", "err", err)
		return nil, toStatusErr(err)
	}

	resp := &assessmentv1.GetNextQuestionResponse{
		Completed: result.Completed,
		Progress:  mappers.AttemptProgressToProto(result.Progress),
	}

	if result.NextQuestion != nil {
		resp.NextQuestion = mappers.QuestionToProto(*result.NextQuestion)
	}

	log.Info("grpc request completed",
		"attempt_id", req.GetAttemptId(),
		"completed", result.Completed,
		"has_next_question", result.NextQuestion != nil,
		"remaining_seconds", result.Progress.RemainingSeconds,
	)

	return resp, nil
}

func (s *Server) SubmitAnswer(
	ctx context.Context,
	req *assessmentv1.SubmitAnswerRequest,
) (*assessmentv1.SubmitAnswerResponse, error) {

	log := s.logger().With(
		"method", "SubmitAnswer",
		"attempt_id", req.GetAttemptId(),
		"question_id", req.GetQuestionId(),
		"selected_option_id", req.GetSelectedOptionId(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	result, err := s.service.SubmitAnswer(ctx, assessmentsvc.SubmitAnswerInput{
		UserID:           userID,
		AttemptID:        req.GetAttemptId(),
		QuestionID:       req.GetQuestionId(),
		SelectedOptionID: req.GetSelectedOptionId(),
	})
	if err != nil {
		log.Error("service.SubmitAnswer failed", "err", err)
		return nil, toStatusErr(err)
	}

	resp := &assessmentv1.SubmitAnswerResponse{
		Completed: result.Completed,
		Progress:  mappers.AttemptProgressToProto(result.Progress),
	}

	if result.NextQuestion != nil {
		resp.NextQuestion = mappers.QuestionToProto(*result.NextQuestion)
	}

	log.Info("grpc request completed",
		"attempt_id", req.GetAttemptId(),
		"completed", result.Completed,
		"has_next_question", result.NextQuestion != nil,
		"sequence_no", result.Progress.SequenceNo,
		"remaining_seconds", result.Progress.RemainingSeconds,
	)

	return resp, nil
}

func (s *Server) FinishAttempt(
	ctx context.Context,
	req *assessmentv1.FinishAttemptRequest,
) (*assessmentv1.Attempt, error) {

	log := s.logger().With(
		"method", "FinishAttempt",
		"attempt_id", req.GetAttemptId(),
		"action", req.GetAction().String(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	attempt, err := s.service.FinishAttempt(ctx, assessmentsvc.FinishAttemptInput{
		UserID:    userID,
		AttemptID: req.GetAttemptId(),
		Action:    mappers.FinishActionFromProto(req.GetAction()),
	})
	if err != nil {
		log.Error("service.FinishAttempt failed", "err", err)
		return nil, toStatusErr(err)
	}

	log.Info("grpc request completed",
		"attempt_id", attempt.ID,
		"assessment_id", attempt.AssessmentSummary.AssessmentID,
		"assessment_code", attempt.AssessmentSummary.AssessmentCode,
		"assessment_title", attempt.AssessmentSummary.AssessmentTitle,
		"subject_id", attempt.AssessmentSummary.SubjectID,
		"subject_code", attempt.AssessmentSummary.SubjectCode,
		"subject_title", attempt.AssessmentSummary.SubjectTitle,
		"assessment_mode", attempt.AssessmentSummary.Mode,
		"status", attempt.Status,
		"finish_reason", attempt.FinishReason,
		"overall_level", attempt.OverallLevel,
		"overall_confidence", attempt.OverallConfidence,
	)

	return mappers.AttemptToProto(*attempt), nil
}

func (s *Server) ListMyAttempts(
	ctx context.Context,
	req *assessmentv1.ListMyAttemptsRequest,
) (*assessmentv1.ListMyAttemptsResponse, error) {
	log := s.logger().With(
		"method", "ListMyAttempts",
		"assessment_id", req.GetAssessmentId(),
		"status", req.GetStatus().String(),
		"page_size", req.GetPageSize(),
		"page_token", req.GetPageToken(),
	)

	userID, err := userIDFromContext(ctx)
	if err != nil {
		log.Error("userIDFromContext failed", "err", err)
		return nil, toStatusErr(err)
	}

	log = log.With("user_id", userID)
	log.Info("grpc request started")

	attempts, nextPageToken, err := s.service.ListMyAttempts(ctx, models.ListMyAttemptsFilter{
		UserID:       userID,
		AssessmentID: req.GetAssessmentId(),
		Status:       mappers.AttemptStatusFromProto(req.GetStatus()),
		PageSize:     int(req.GetPageSize()),
		PageToken:    req.GetPageToken(),
	})
	if err != nil {
		log.Error("service.ListMyAttempts failed", "err", err)
		return nil, toStatusErr(err)
	}

	items := make([]*assessmentv1.Attempt, 0, len(attempts))

	globalCount := 0
	subtopicCount := 0

	for _, attempt := range attempts {
		items = append(items, mappers.AttemptToProto(attempt))

		switch attempt.AssessmentSummary.Mode {
		case models.AssessmentModeGlobal:
			globalCount++
		case models.AssessmentModeSubtopic:
			subtopicCount++
		}
	}

	log.Info("grpc request completed",
		"items_count", len(items),
		"global_count", globalCount,
		"subtopic_count", subtopicCount,
		"next_page_token", nextPageToken,
	)

	return &assessmentv1.ListMyAttemptsResponse{
		Attempts:      items,
		NextPageToken: nextPageToken,
	}, nil
}
