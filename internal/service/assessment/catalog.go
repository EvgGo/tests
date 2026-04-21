package assessmentsvc

import (
	"context"
	"testing/internal/models"
)

func (s *Service) ListSubjects(
	ctx context.Context,
	filter models.ListSubjectsFilter,
) ([]models.Subject, string, error) {
	return s.catalog.ListSubjects(ctx, filter)
}

func (s *Service) ListSubtopics(
	ctx context.Context,
	filter models.ListSubtopicsFilter,
) ([]models.Subtopic, string, error) {
	return s.catalog.ListSubtopics(ctx, filter)
}

func (s *Service) ListAssessments(
	ctx context.Context,
	filter models.ListAssessmentsFilter,
) ([]models.AssessmentSummary, string, error) {
	return s.catalog.ListAssessments(ctx, filter)
}

func (s *Service) GetAssessment(
	ctx context.Context,
	assessmentID int64,
	includeSubtopics bool,
) (*models.Assessment, error) {
	return s.catalog.GetAssessment(ctx, assessmentID, includeSubtopics)
}

func (s *Service) ListMyAttempts(
	ctx context.Context,
	filter models.ListMyAttemptsFilter,
) ([]models.Attempt, string, error) {
	filter.UserID = normalizeUserID(filter.UserID)

	return s.attempts.ListMyAttempts(ctx, filter)
}
