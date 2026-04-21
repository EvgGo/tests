package postgres

import (
	"context"
	"errors"
	"strings"
	"testing/internal/models"
	"testing/internal/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttemptRepo struct {
	pool *pgxpool.Pool
}

func NewAttemptRepo(pool *pgxpool.Pool) *AttemptRepo {
	return &AttemptRepo{pool: pool}
}

func (r *AttemptRepo) FindInProgressAttempt(
	ctx context.Context,
	userID string,
	assessmentID int64,
) (*models.Attempt, error) {
	userID = strings.TrimSpace(userID)

	if userID == "" || assessmentID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			a.id,
			a.assessment_id,
			a.user_id,
			a.status,
			a.finish_reason,
			a.sequence_no,
			a.started_at,
			a.expires_at,
			a.completed_at,
			a.duration_seconds,
			a.overall_level,
			a.overall_level_score,
			a.overall_confidence,
			a.created_at,
			a.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from attempts a
		join assessments ass on ass.id = a.assessment_id
		join subjects subj on subj.id = ass.subject_id
		where a.user_id = $1
		  and a.assessment_id = $2
		  and a.status = 'in_progress'
		limit 1
	`

	attempt, err := scanAttempt(qr.QueryRow(ctx, query, userID, assessmentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return attempt, nil
}

func (r *AttemptRepo) CreateAttempt(
	ctx context.Context,
	input models.CreateAttemptInput,
) (*models.Attempt, error) {

	input.UserID = strings.TrimSpace(input.UserID)

	if input.UserID == "" || input.AssessmentID <= 0 || input.DurationSeconds < 30 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		with inserted as (
			insert into attempts (
				assessment_id,
				user_id,
				status,
				finish_reason,
				sequence_no,
				started_at,
				expires_at,
				duration_seconds,
				overall_level,
				overall_level_score,
				overall_confidence
			)
			values (
				$1,
				$2,
				'in_progress',
				'unspecified',
				0,
				$3,
				$4,
				$5,
				0,
				0.00,
				0.000
			)
			returning
				id,
				assessment_id,
				user_id,
				status,
				finish_reason,
				sequence_no,
				started_at,
				expires_at,
				completed_at,
				duration_seconds,
				overall_level,
				overall_level_score,
				overall_confidence,
				created_at,
				updated_at
		)
		select
			i.id,
			i.assessment_id,
			i.user_id,
			i.status,
			i.finish_reason,
			i.sequence_no,
			i.started_at,
			i.expires_at,
			i.completed_at,
			i.duration_seconds,
			i.overall_level,
			i.overall_level_score,
			i.overall_confidence,
			i.created_at,
			i.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from inserted i
		join assessments ass on ass.id = i.assessment_id
		join subjects subj on subj.id = ass.subject_id
	`

	attempt, err := scanAttempt(qr.QueryRow(
		ctx,
		query,
		input.AssessmentID,
		input.UserID,
		input.StartedAt,
		input.ExpiresAt,
		input.DurationSeconds,
	))
	if isUniqueViolation(err) {
		return nil, repo.ErrAlreadyExists
	}
	if err != nil {
		return nil, err
	}

	return attempt, nil
}

func (r *AttemptRepo) GetAttempt(
	ctx context.Context,
	attemptID int64,
	userID string,
	includeSubtopicStates bool,
) (*models.Attempt, error) {

	userID = strings.TrimSpace(userID)

	if attemptID <= 0 || userID == "" {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			a.id,
			a.assessment_id,
			a.user_id,
			a.status,
			a.finish_reason,
			a.sequence_no,
			a.started_at,
			a.expires_at,
			a.completed_at,
			a.duration_seconds,
			a.overall_level,
			a.overall_level_score,
			a.overall_confidence,
			a.created_at,
			a.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from attempts a
		join assessments ass on ass.id = a.assessment_id
		join subjects subj on subj.id = ass.subject_id
		where a.id = $1
		  and a.user_id = $2
	`

	attempt, err := scanAttempt(qr.QueryRow(ctx, query, attemptID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if includeSubtopicStates {
		states, err := r.ListAttemptStates(ctx, attempt.ID)
		if err != nil {
			return nil, err
		}

		attempt.SubtopicStates = states
	}

	return attempt, nil
}

func (r *AttemptRepo) GetAttemptForUpdate(
	ctx context.Context,
	attemptID int64,
	userID string,
) (*models.Attempt, error) {

	userID = strings.TrimSpace(userID)

	if attemptID <= 0 || userID == "" {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			a.id,
			a.assessment_id,
			a.user_id,
			a.status,
			a.finish_reason,
			a.sequence_no,
			a.started_at,
			a.expires_at,
			a.completed_at,
			a.duration_seconds,
			a.overall_level,
			a.overall_level_score,
			a.overall_confidence,
			a.created_at,
			a.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from attempts a
		join assessments ass on ass.id = a.assessment_id
		join subjects subj on subj.id = ass.subject_id
		where a.id = $1
		  and a.user_id = $2
		for update
	`

	attempt, err := scanAttempt(qr.QueryRow(ctx, query, attemptID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return attempt, nil
}

func (r *AttemptRepo) ListAttemptStates(
	ctx context.Context,
	attemptID int64,
) ([]models.AttemptSubtopicState, error) {
	return r.listAttemptStates(ctx, attemptID, false)
}

func (r *AttemptRepo) ListAttemptStatesForUpdate(
	ctx context.Context,
	attemptID int64,
) ([]models.AttemptSubtopicState, error) {
	return r.listAttemptStates(ctx, attemptID, true)
}

func (r *AttemptRepo) UpdateAttemptState(
	ctx context.Context,
	input models.UpdateAttemptStateInput,
) error {
	if input.AttemptID <= 0 || input.SubtopicID <= 0 {
		return repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		update attempt_subtopic_state
		set
			asked_count = $3,
			correct_count = $4,
			wrong_count = $5,
			consecutive_correct = $6,
			consecutive_wrong = $7,
			level_score = $8,
			estimated_level = $9,
			confidence = $10,
			is_locked = $11,
			last_question_id = $12,
			last_answer_correct = $13
		where attempt_id = $1
		  and subtopic_id = $2
	`

	tag, err := qr.Exec(
		ctx,
		query,
		input.AttemptID,
		input.SubtopicID,
		input.AskedCount,
		input.CorrectCount,
		input.WrongCount,
		input.ConsecutiveCorrect,
		input.ConsecutiveWrong,
		input.LevelScore,
		input.EstimatedLevel,
		input.Confidence,
		input.IsLocked,
		input.LastQuestionID,
		input.LastAnswerCorrect,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}

	return nil
}

func (r *AttemptRepo) CreateAttemptAnswer(
	ctx context.Context,
	input models.CreateAttemptAnswerInput,
) error {
	if input.AttemptID <= 0 || input.SeqNo <= 0 || input.QuestionID <= 0 || input.SubtopicID <= 0 || input.SelectedOptionID <= 0 {
		return repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		insert into attempt_answers (
			attempt_id,
			seq_no,
			question_id,
			subtopic_id,
			difficulty,
			selected_option_id,
			is_correct
		)
		values (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7
		)
	`

	_, err := qr.Exec(
		ctx,
		query,
		input.AttemptID,
		input.SeqNo,
		input.QuestionID,
		input.SubtopicID,
		input.Difficulty,
		input.SelectedOptionID,
		input.IsCorrect,
	)
	if isUniqueViolation(err) {
		return repo.ErrAlreadyExists
	}
	if err != nil {
		return err
	}

	return nil
}

func (r *AttemptRepo) UpdateAttemptAggregate(
	ctx context.Context,
	input models.UpdateAttemptAggregateInput,
) error {
	if input.AttemptID <= 0 || input.SequenceNo < 0 {
		return repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		update attempts
		set
			sequence_no = $2,
			overall_level = $3,
			overall_level_score = $4,
			overall_confidence = $5
		where id = $1
	`

	tag, err := qr.Exec(
		ctx,
		query,
		input.AttemptID,
		input.SequenceNo,
		input.OverallLevel,
		input.OverallLevelScore,
		input.OverallConfidence,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repo.ErrNotFound
	}

	return nil
}

func (r *AttemptRepo) FinishAttempt(
	ctx context.Context,
	input models.FinishAttemptInput,
) (*models.Attempt, error) {

	if input.AttemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		with updated as (
			update attempts
			set
				status = $2,
				finish_reason = $3,
				completed_at = $4,
				overall_level = $5,
				overall_level_score = $6,
				overall_confidence = $7
			where id = $1
			returning
				id,
				assessment_id,
				user_id,
				status,
				finish_reason,
				sequence_no,
				started_at,
				expires_at,
				completed_at,
				duration_seconds,
				overall_level,
				overall_level_score,
				overall_confidence,
				created_at,
				updated_at
		)
		select
			u.id,
			u.assessment_id,
			u.user_id,
			u.status,
			u.finish_reason,
			u.sequence_no,
			u.started_at,
			u.expires_at,
			u.completed_at,
			u.duration_seconds,
			u.overall_level,
			u.overall_level_score,
			u.overall_confidence,
			u.created_at,
			u.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from updated u
		join assessments ass on ass.id = u.assessment_id
		join subjects subj on subj.id = ass.subject_id
	`

	attempt, err := scanAttempt(qr.QueryRow(
		ctx,
		query,
		input.AttemptID,
		string(input.Status),
		string(input.FinishReason),
		input.CompletedAt,
		input.OverallLevel,
		input.OverallLevelScore,
		input.OverallConfidence,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return attempt, nil
}

func (r *AttemptRepo) ListMyAttempts(
	ctx context.Context,
	filter models.ListMyAttemptsFilter,
) ([]models.Attempt, string, error) {
	filter.UserID = strings.TrimSpace(filter.UserID)

	if filter.UserID == "" {
		return nil, "", repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	pageSize := normalizePageSize(filter.PageSize, 20, 100)

	cursorID, err := decodeIDCursor(filter.PageToken)
	if err != nil {
		return nil, "", err
	}

	status := string(filter.Status)

	query := `
		select
			a.id,
			a.assessment_id,
			a.user_id,
			a.status,
			a.finish_reason,
			a.sequence_no,
			a.started_at,
			a.expires_at,
			a.completed_at,
			a.duration_seconds,
			a.overall_level,
			a.overall_level_score,
			a.overall_confidence,
			a.created_at,
			a.updated_at,

			ass.code,
			ass.title,
			subj.id,
			subj.code,
			subj.title,
			ass.mode
		from attempts a
		join assessments ass on ass.id = a.assessment_id
		join subjects subj on subj.id = ass.subject_id
		where a.user_id = $1
		  and a.id > $2
		  and ($3::bigint = 0 or a.assessment_id = $3)
		  and ($4 = '' or a.status = $4)
		order by a.id asc
		limit $5
	`

	rows, err := qr.Query(ctx, query, filter.UserID, cursorID, filter.AssessmentID, status, pageSize+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	attempts := make([]models.Attempt, 0, pageSize+1)

	for rows.Next() {
		attempt, err := scanAttemptRows(rows)
		if err != nil {
			return nil, "", err
		}

		attempts = append(attempts, *attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextPageToken := ""
	if len(attempts) > pageSize {
		nextPageToken = encodeIDCursor(attempts[pageSize-1].ID)
		attempts = attempts[:pageSize]
	}

	return attempts, nextPageToken, nil
}

func (r *AttemptRepo) listAttemptStates(
	ctx context.Context,
	attemptID int64,
	forUpdate bool,
) ([]models.AttemptSubtopicState, error) {
	if attemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			attempt_id,
			subtopic_id,
			asked_count,
			correct_count,
			wrong_count,
			consecutive_correct,
			consecutive_wrong,
			level_score,
			estimated_level,
			confidence,
			is_locked,
			last_question_id,
			last_answer_correct,
			weight,
			priority,
			min_items,
			max_items,
			start_difficulty,
			stop_confidence,
			created_at,
			updated_at
		from attempt_subtopic_state
		where attempt_id = $1
		order by
			case when asked_count < min_items then 0 else 1 end asc,
			confidence asc,
			weight desc,
			asked_count asc,
			priority asc,
			subtopic_id asc
	`

	if forUpdate {
		query += " for update"
	}

	rows, err := qr.Query(ctx, query, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make([]models.AttemptSubtopicState, 0)

	for rows.Next() {
		var state models.AttemptSubtopicState

		if err := rows.Scan(
			&state.AttemptID,
			&state.SubtopicID,
			&state.AskedCount,
			&state.CorrectCount,
			&state.WrongCount,
			&state.ConsecutiveCorrect,
			&state.ConsecutiveWrong,
			&state.LevelScore,
			&state.EstimatedLevel,
			&state.Confidence,
			&state.IsLocked,
			&state.LastQuestionID,
			&state.LastAnswerCorrect,
			&state.Weight,
			&state.Priority,
			&state.MinItems,
			&state.MaxItems,
			&state.StartDifficulty,
			&state.StopConfidence,
			&state.CreatedAt,
			&state.UpdatedAt,
		); err != nil {
			return nil, err
		}

		states = append(states, state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

func scanAttempt(row pgx.Row) (*models.Attempt, error) {
	var attempt models.Attempt
	var status string
	var finishReason string
	var assessmentCode string
	var assessmentTitle string
	var subjectID int64
	var subjectCode string
	var subjectTitle string
	var assessmentMode string

	err := row.Scan(
		&attempt.ID,
		&attempt.AssessmentID,
		&attempt.UserID,
		&status,
		&finishReason,
		&attempt.SequenceNo,
		&attempt.StartedAt,
		&attempt.ExpiresAt,
		&attempt.CompletedAt,
		&attempt.DurationSeconds,
		&attempt.OverallLevel,
		&attempt.OverallLevelScore,
		&attempt.OverallConfidence,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,

		&assessmentCode,
		&assessmentTitle,
		&subjectID,
		&subjectCode,
		&subjectTitle,
		&assessmentMode,
	)
	if err != nil {
		return nil, err
	}

	attempt.Status = models.AttemptStatus(status)
	attempt.FinishReason = models.FinishReason(finishReason)

	attempt.AssessmentSummary = models.AttemptAssessmentSummary{
		AssessmentID:    attempt.AssessmentID,
		AssessmentCode:  assessmentCode,
		AssessmentTitle: assessmentTitle,
		SubjectID:       subjectID,
		SubjectCode:     subjectCode,
		SubjectTitle:    subjectTitle,
		Mode:            models.AssessmentMode(assessmentMode),
	}

	return &attempt, nil
}

func (r *AttemptRepo) CreateAttemptState(
	ctx context.Context,
	input models.CreateAttemptStateInput,
) error {
	if input.AttemptID <= 0 || input.SubtopicID <= 0 {
		return repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	const query = `
		insert into attempt_subtopic_state (
			attempt_id,
			subtopic_id,
			asked_count,
			correct_count,
			wrong_count,
			consecutive_correct,
			consecutive_wrong,
			level_score,
			estimated_level,
			confidence,
			is_locked,
			weight,
			priority,
			min_items,
			max_items,
			start_difficulty,
			stop_confidence
		)
		values (
			$1,
			$2,
			0,
			0,
			0,
			0,
			0,
			$3,
			$4,
			$5,
			false,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11
		)
	`

	_, err := qr.Exec(
		ctx,
		query,
		input.AttemptID,
		input.SubtopicID,
		input.LevelScore,
		input.EstimatedLevel,
		input.Confidence,
		input.Weight,
		input.Priority,
		input.MinItems,
		input.MaxItems,
		input.StartDifficulty,
		input.StopConfidence,
	)
	if isUniqueViolation(err) {
		return repo.ErrAlreadyExists
	}
	if err != nil {
		return err
	}

	return nil
}

func scanAttemptRows(rows pgx.Rows) (*models.Attempt, error) {
	var attempt models.Attempt
	var status string
	var finishReason string
	var assessmentCode string
	var assessmentTitle string
	var subjectID int64
	var subjectCode string
	var subjectTitle string
	var assessmentMode string

	err := rows.Scan(
		&attempt.ID,
		&attempt.AssessmentID,
		&attempt.UserID,
		&status,
		&finishReason,
		&attempt.SequenceNo,
		&attempt.StartedAt,
		&attempt.ExpiresAt,
		&attempt.CompletedAt,
		&attempt.DurationSeconds,
		&attempt.OverallLevel,
		&attempt.OverallLevelScore,
		&attempt.OverallConfidence,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,

		&assessmentCode,
		&assessmentTitle,
		&subjectID,
		&subjectCode,
		&subjectTitle,
		&assessmentMode,
	)
	if err != nil {
		return nil, err
	}

	attempt.Status = models.AttemptStatus(status)
	attempt.FinishReason = models.FinishReason(finishReason)

	attempt.AssessmentSummary = models.AttemptAssessmentSummary{
		AssessmentID:    attempt.AssessmentID,
		AssessmentCode:  assessmentCode,
		AssessmentTitle: assessmentTitle,
		SubjectID:       subjectID,
		SubjectCode:     subjectCode,
		SubjectTitle:    subjectTitle,
		Mode:            models.AssessmentMode(assessmentMode),
	}

	return &attempt, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
