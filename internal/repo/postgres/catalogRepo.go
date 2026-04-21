package postgres

import (
	"context"
	"errors"
	"strings"
	"testing/internal/models"
	"testing/internal/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepo struct {
	pool *pgxpool.Pool
}

func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo {
	return &CatalogRepo{pool: pool}
}

func (r *CatalogRepo) ListSubjects(
	ctx context.Context,
	filter models.ListSubjectsFilter,
) ([]models.Subject, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := normalizePageSize(filter.PageSize, 20, 100)

	cursorID, err := decodeIDCursor(filter.PageToken)
	if err != nil {
		return nil, "", err
	}

	query := `
		select
			id,
			code,
			title,
			created_at
		from subjects
		where id > $1
		  and (
			$2 = ''
			or code ilike '%' || $2 || '%'
			or title ilike '%' || $2 || '%'
		  )
		order by id asc
		limit $3
	`

	rows, err := qr.Query(ctx, query, cursorID, strings.TrimSpace(filter.Query), pageSize+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	subjects := make([]models.Subject, 0, pageSize+1)

	for rows.Next() {
		var subject models.Subject

		if err = rows.Scan(
			&subject.ID,
			&subject.Code,
			&subject.Title,
			&subject.CreatedAt,
		); err != nil {
			return nil, "", err
		}

		subjects = append(subjects, subject)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if len(subjects) > pageSize {
		nextPageToken = encodeIDCursor(subjects[pageSize-1].ID)
		subjects = subjects[:pageSize]
	}

	return subjects, nextPageToken, nil
}

func (r *CatalogRepo) ListSubtopics(
	ctx context.Context,
	filter models.ListSubtopicsFilter,
) ([]models.Subtopic, string, error) {

	qr := querierFromCtx(ctx, r.pool)

	pageSize := normalizePageSize(filter.PageSize, 20, 100)

	cursorID, err := decodeIDCursor(filter.PageToken)
	if err != nil {
		return nil, "", err
	}

	query := `
		select
			id,
			subject_id,
			code,
			title,
			created_at
		from subtopics
		where id > $1
		  and ($2::bigint = 0 or subject_id = $2)
		  and (
			$3 = ''
			or code ilike '%' || $3 || '%'
			or title ilike '%' || $3 || '%'
		  )
		order by id asc
		limit $4
	`

	rows, err := qr.Query(ctx, query, cursorID, filter.SubjectID, strings.TrimSpace(filter.Query), pageSize+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	subtopics := make([]models.Subtopic, 0, pageSize+1)

	for rows.Next() {
		var subtopic models.Subtopic

		if err = rows.Scan(
			&subtopic.ID,
			&subtopic.SubjectID,
			&subtopic.Code,
			&subtopic.Title,
			&subtopic.CreatedAt,
		); err != nil {
			return nil, "", err
		}

		subtopics = append(subtopics, subtopic)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if len(subtopics) > pageSize {
		nextPageToken = encodeIDCursor(subtopics[pageSize-1].ID)
		subtopics = subtopics[:pageSize]
	}

	return subtopics, nextPageToken, nil
}

func (r *CatalogRepo) ListAssessments(
	ctx context.Context,
	filter models.ListAssessmentsFilter,
) ([]models.AssessmentSummary, string, error) {
	qr := querierFromCtx(ctx, r.pool)

	pageSize := normalizePageSize(filter.PageSize, 20, 100)

	cursorID, err := decodeIDCursor(filter.PageToken)
	if err != nil {
		return nil, "", err
	}

	status := string(filter.Status)

	query := `
		select
			id,
			subject_id,
			code,
			title,
			coalesce(description, '') as description,
			status,
			mode,
			duration_seconds
		from assessments
		where id > $1
		  and ($2::bigint = 0 or subject_id = $2)
		  and ($3 = '' or status = $3)
		  and (
			$4 = ''
			or code ilike '%' || $4 || '%'
			or title ilike '%' || $4 || '%'
		  )
		order by id asc
		limit $5
	`

	rows, err := qr.Query(ctx, query, cursorID, filter.SubjectID, status, strings.TrimSpace(filter.Query), pageSize+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	assessments := make([]models.AssessmentSummary, 0, pageSize+1)

	for rows.Next() {
		var assessment models.AssessmentSummary
		var assessmentStatus string
		var assessmentMode string

		if err := rows.Scan(
			&assessment.ID,
			&assessment.SubjectID,
			&assessment.Code,
			&assessment.Title,
			&assessment.Description,
			&assessmentStatus,
			&assessmentMode,
			&assessment.DurationSeconds,
		); err != nil {
			return nil, "", err
		}

		assessment.Status = models.AssessmentStatus(assessmentStatus)
		assessment.Mode = models.AssessmentMode(assessmentMode)

		assessments = append(assessments, assessment)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if len(assessments) > pageSize {
		nextPageToken = encodeIDCursor(assessments[pageSize-1].ID)
		assessments = assessments[:pageSize]
	}

	return assessments, nextPageToken, nil
}

func (r *CatalogRepo) GetAssessment(
	ctx context.Context,
	assessmentID int64,
	includeSubtopics bool,
) (*models.Assessment, error) {

	if assessmentID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	assessment, err := r.getAssessmentBase(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	if includeSubtopics {
		subtopics, err := r.listAssessmentSubtopicConfigs(ctx, assessmentID)
		if err != nil {
			return nil, err
		}

		assessment.Subtopics = subtopics
	}

	return assessment, nil
}

func (r *CatalogRepo) GetAssessmentForStart(
	ctx context.Context,
	assessmentID int64,
) (*models.Assessment, error) {

	if assessmentID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	assessment, err := r.getAssessmentBase(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	if assessment.Status != models.AssessmentStatusActive {
		return nil, repo.ErrInvalidState
	}

	subtopics, err := r.listAssessmentSubtopicConfigs(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	if len(subtopics) == 0 {
		return nil, repo.ErrInvalidState
	}

	assessment.Subtopics = subtopics

	return assessment, nil
}

func (r *CatalogRepo) getAssessmentBase(
	ctx context.Context,
	assessmentID int64,
) (*models.Assessment, error) {
	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			id,
			subject_id,
			code,
			title,
			coalesce(description, '') as description,
			status,
			mode,
			min_items,
			max_items,
			min_difficulty,
			max_difficulty,
			start_difficulty,
			stop_confidence,
			duration_seconds,
			created_at,
			updated_at
		from assessments
		where id = $1
	`

	var assessment models.Assessment
	var status string
	var mode string

	err := qr.QueryRow(ctx, query, assessmentID).Scan(
		&assessment.ID,
		&assessment.SubjectID,
		&assessment.Code,
		&assessment.Title,
		&assessment.Description,
		&status,
		&mode,
		&assessment.MinItems,
		&assessment.MaxItems,
		&assessment.MinDifficulty,
		&assessment.MaxDifficulty,
		&assessment.StartDifficulty,
		&assessment.StopConfidence,
		&assessment.DurationSeconds,
		&assessment.CreatedAt,
		&assessment.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	assessment.Status = models.AssessmentStatus(status)
	assessment.Mode = models.AssessmentMode(mode)

	return &assessment, nil
}

func (r *CatalogRepo) listAssessmentSubtopicConfigs(
	ctx context.Context,
	assessmentID int64,
) ([]models.AssessmentSubtopicConfig, error) {

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			assessment_id,
			subtopic_id,
			weight,
			priority,
			min_items,
			max_items,
			start_difficulty,
			stop_confidence
		from assessment_subtopics
		where assessment_id = $1
		order by priority asc, subtopic_id asc
	`

	rows, err := qr.Query(ctx, query, assessmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := make([]models.AssessmentSubtopicConfig, 0)

	for rows.Next() {
		var config models.AssessmentSubtopicConfig

		if err = rows.Scan(
			&config.AssessmentID,
			&config.SubtopicID,
			&config.Weight,
			&config.Priority,
			&config.MinItems,
			&config.MaxItems,
			&config.StartDifficulty,
			&config.StopConfidence,
		); err != nil {
			return nil, err
		}

		configs = append(configs, config)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return configs, nil
}
