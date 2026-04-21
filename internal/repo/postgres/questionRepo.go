package postgres

import (
	"context"
	"errors"
	"testing/internal/models"
	"testing/internal/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuestionRepo struct {
	pool *pgxpool.Pool
}

func NewQuestionRepo(pool *pgxpool.Pool) *QuestionRepo {
	return &QuestionRepo{pool: pool}
}

func (r *QuestionRepo) GetQuestionForAnswer(
	ctx context.Context,
	questionID int64,
) (*models.Question, error) {

	if questionID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			id,
			subtopic_id,
			difficulty,
			body,
			correct_option_id
		from questions
		where id = $1
		  and status = 'published'
	`

	var question models.Question

	err := qr.QueryRow(ctx, query, questionID).Scan(
		&question.ID,
		&question.SubtopicID,
		&question.Difficulty,
		&question.Body,
		&question.CorrectOptionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if question.CorrectOptionID == nil {
		return nil, repo.ErrInvalidState
	}

	return &question, nil
}

func (r *QuestionRepo) GetQuestionWithOptions(
	ctx context.Context,
	questionID int64,
) (*models.Question, error) {

	question, err := r.getQuestionBase(ctx, questionID)
	if err != nil {
		return nil, err
	}

	options, err := r.listQuestionOptions(ctx, questionID)
	if err != nil {
		return nil, err
	}

	question.Options = options

	return question, nil
}

func (r *QuestionRepo) OptionBelongsToQuestion(
	ctx context.Context,
	questionID int64,
	optionID int64,
) (bool, error) {

	if questionID <= 0 || optionID <= 0 {
		return false, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select exists(
			select 1
			from question_options
			where id = $1
			  and question_id = $2
		)
	`

	var existsValue bool

	if err := qr.QueryRow(ctx, query, optionID, questionID).Scan(&existsValue); err != nil {
		return false, err
	}

	return existsValue, nil
}

func (r *QuestionRepo) FindNextQuestionByDifficulties(
	ctx context.Context,
	attemptID int64,
	subtopicID int64,
	difficulties []int,
) (*models.Question, error) {

	if attemptID <= 0 || subtopicID <= 0 || len(difficulties) == 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		with difficulty_order as (
			select
				value::smallint as difficulty,
				ord::int as ord
			from unnest($3::int[]) with ordinality as t(value, ord)
		)
		select
			q.id,
			q.subtopic_id,
			q.difficulty,
			q.body,
			q.correct_option_id
		from questions q
		join difficulty_order d on d.difficulty = q.difficulty
		where q.subtopic_id = $2
		  and q.status = 'published'
		  and not exists (
			select 1
			from attempt_answers aa
			where aa.attempt_id = $1
			  and aa.question_id = q.id
		  )
		order by d.ord asc, q.id asc
		limit 1
	`

	var question models.Question

	err := qr.QueryRow(ctx, query, attemptID, subtopicID, difficulties).Scan(
		&question.ID,
		&question.SubtopicID,
		&question.Difficulty,
		&question.Body,
		&question.CorrectOptionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNoQuestions
	}
	if err != nil {
		return nil, err
	}

	options, err := r.listQuestionOptions(ctx, question.ID)
	if err != nil {
		return nil, err
	}

	question.Options = options

	return &question, nil
}

func (r *QuestionRepo) getQuestionBase(
	ctx context.Context,
	questionID int64,
) (*models.Question, error) {

	if questionID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			id,
			subtopic_id,
			difficulty,
			body,
			correct_option_id
		from questions
		where id = $1
		  and status = 'published'
	`

	var question models.Question

	err := qr.QueryRow(ctx, query, questionID).Scan(
		&question.ID,
		&question.SubtopicID,
		&question.Difficulty,
		&question.Body,
		&question.CorrectOptionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &question, nil
}

func (r *QuestionRepo) listQuestionOptions(
	ctx context.Context,
	questionID int64,
) ([]models.QuestionOption, error) {

	qr := querierFromCtx(ctx, r.pool)

	query := `
		select
			id,
			question_id,
			body,
			position
		from question_options
		where question_id = $1
		order by position asc, id asc
	`

	rows, err := qr.Query(ctx, query, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	options := make([]models.QuestionOption, 0)

	for rows.Next() {
		var option models.QuestionOption

		if err = rows.Scan(
			&option.ID,
			&option.QuestionID,
			&option.Body,
			&option.Position,
		); err != nil {
			return nil, err
		}

		options = append(options, option)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return options, nil
}

func (r *QuestionRepo) FindNextGlobalQuestionByDifficulties(
	ctx context.Context,
	attemptID int64,
	subjectID int64,
	difficulties []int,
) (*models.Question, error) {

	if attemptID <= 0 || subjectID <= 0 || len(difficulties) == 0 {
		return nil, repo.ErrInvalidInput
	}

	qr := querierFromCtx(ctx, r.pool)

	query := `
		with difficulty_order as (
			select
				value::smallint as difficulty,
				ord::int as ord
			from unnest($3::int[]) with ordinality as t(value, ord)
		)
		select
			q.id,
			q.subtopic_id,
			q.difficulty,
			q.body,
			q.correct_option_id
		from questions q
		join subtopics s on s.id = q.subtopic_id
		join difficulty_order d on d.difficulty = q.difficulty
		where s.subject_id = $2
		  and q.status = 'published'
		  and not exists (
			select 1
			from attempt_answers aa
			where aa.attempt_id = $1
			  and aa.question_id = q.id
		  )
		order by d.ord asc, q.id asc
		limit 1
	`

	var question models.Question

	err := qr.QueryRow(ctx, query, attemptID, subjectID, difficulties).Scan(
		&question.ID,
		&question.SubtopicID,
		&question.Difficulty,
		&question.Body,
		&question.CorrectOptionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repo.ErrNoQuestions
	}
	if err != nil {
		return nil, err
	}

	options, err := r.listQuestionOptions(ctx, question.ID)
	if err != nil {
		return nil, err
	}

	question.Options = options

	return &question, nil
}
