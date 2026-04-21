package assessmentsvc

import (
	"context"
	"errors"
	"testing/internal/models"
	"testing/internal/repo"
)

type selectedQuestion struct {
	Question *models.Question
	State    models.AttemptSubtopicState
}

// pickNextQuestionLocked выбирает следующий вопрос
// функция может залочить подтему, если по ней больше нет доступных вопросов
func (s *Service) pickNextQuestionLocked(
	ctx context.Context,
	attemptID int64,
	states []models.AttemptSubtopicState,
	minDifficulty int,
	maxDifficulty int,
) (*selectedQuestion, []models.AttemptSubtopicState, error) {

	for i := range states {
		state := states[i]

		if state.IsLocked {
			continue
		}

		if state.AskedCount >= state.MaxItems {
			state.IsLocked = true

			if err := s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
				return nil, states, err
			}

			states[i] = state
			continue
		}

		normalOrder := BuildDifficultyOrder(state, minDifficulty, maxDifficulty)

		question, err := s.questions.FindNextQuestionByDifficulties(
			ctx,
			attemptID,
			state.SubtopicID,
			normalOrder,
		)
		if err == nil {
			return &selectedQuestion{
				Question: question,
				State:    state,
			}, states, nil
		}

		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return nil, states, err
		}

		expandedOrder := BuildExpandedDifficultyOrder(state, minDifficulty, maxDifficulty)

		question, err = s.questions.FindNextQuestionByDifficulties(
			ctx,
			attemptID,
			state.SubtopicID,
			expandedOrder,
		)
		if err == nil {
			return &selectedQuestion{
				Question: question,
				State:    state,
			}, states, nil
		}

		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return nil, states, err
		}

		// Если даже расширенный поиск ничего не дал,
		// подтему закрываем и идем к следующей
		state.IsLocked = true

		if err = s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
			return nil, states, err
		}

		states[i] = state
	}

	return nil, states, repo.ErrNoQuestions
}

func (s *Service) pickNextQuestionAfterAnswerLocked(
	ctx context.Context,
	attemptID int64,
	states []models.AttemptSubtopicState,
	minDifficulty int,
	maxDifficulty int,
	lastSubtopicID int64,
	lastQuestionDifficulty int,
	lastAnswerCorrect bool,
) (*selectedQuestion, []models.AttemptSubtopicState, error) {

	for i := range states {
		state := states[i]

		if state.IsLocked {
			continue
		}

		if state.AskedCount >= state.MaxItems {
			state.IsLocked = true

			if err := s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
				return nil, states, err
			}

			states[i] = state
			continue
		}

		var normalOrder []int
		var expandedOrder []int

		// Если это та же подтема, по которой только что был ответ,
		// используем более точную логику, учитывающую lastQuestionDifficulty
		if state.SubtopicID == lastSubtopicID {
			normalOrder = BuildDifficultyOrderFromLastResult(
				state,
				lastQuestionDifficulty,
				lastAnswerCorrect,
				minDifficulty,
				maxDifficulty,
			)

			expandedOrder = BuildExpandedDifficultyOrderFromLastResult(
				state,
				lastQuestionDifficulty,
				lastAnswerCorrect,
				minDifficulty,
				maxDifficulty,
			)
		} else {
			normalOrder = BuildDifficultyOrder(state, minDifficulty, maxDifficulty)
			expandedOrder = BuildExpandedDifficultyOrder(state, minDifficulty, maxDifficulty)
		}

		question, err := s.questions.FindNextQuestionByDifficulties(
			ctx,
			attemptID,
			state.SubtopicID,
			normalOrder,
		)
		if err == nil {
			return &selectedQuestion{
				Question: question,
				State:    state,
			}, states, nil
		}

		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return nil, states, err
		}

		question, err = s.questions.FindNextQuestionByDifficulties(
			ctx,
			attemptID,
			state.SubtopicID,
			expandedOrder,
		)
		if err == nil {
			return &selectedQuestion{
				Question: question,
				State:    state,
			}, states, nil
		}

		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return nil, states, err
		}

		state.IsLocked = true

		if err = s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
			return nil, states, err
		}

		states[i] = state
	}

	return nil, states, repo.ErrNoQuestions
}

func (s *Service) pickNextGlobalQuestionLocked(
	ctx context.Context,
	attemptID int64,
	subjectID int64,
	state models.AttemptSubtopicState,
	minDifficulty int,
	maxDifficulty int,
) (*selectedQuestion, models.AttemptSubtopicState, error) {

	if state.IsLocked {
		return nil, state, repo.ErrNoQuestions
	}

	if state.AskedCount >= state.MaxItems {
		state.IsLocked = true

		if err := s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
			return nil, state, err
		}

		return nil, state, repo.ErrNoQuestions
	}

	normalOrder := BuildDifficultyOrder(state, minDifficulty, maxDifficulty)

	question, err := s.questions.FindNextGlobalQuestionByDifficulties(
		ctx,
		attemptID,
		subjectID,
		normalOrder,
	)
	if err == nil {
		return &selectedQuestion{
			Question: question,
			State:    state,
		}, state, nil
	}

	if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
		return nil, state, err
	}

	expandedOrder := BuildExpandedDifficultyOrder(state, minDifficulty, maxDifficulty)

	question, err = s.questions.FindNextGlobalQuestionByDifficulties(
		ctx,
		attemptID,
		subjectID,
		expandedOrder,
	)
	if err == nil {
		return &selectedQuestion{
			Question: question,
			State:    state,
		}, state, nil
	}

	if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
		return nil, state, err
	}

	state.IsLocked = true

	if err = s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
		return nil, state, err
	}

	return nil, state, repo.ErrNoQuestions
}

func (s *Service) pickNextGlobalQuestionAfterAnswerLocked(
	ctx context.Context,
	attemptID int64,
	subjectID int64,
	state models.AttemptSubtopicState,
	minDifficulty int,
	maxDifficulty int,
	lastQuestionDifficulty int,
	lastAnswerCorrect bool,
) (*selectedQuestion, models.AttemptSubtopicState, error) {

	if state.IsLocked {
		return nil, state, repo.ErrNoQuestions
	}

	if state.AskedCount >= state.MaxItems {
		state.IsLocked = true

		if err := s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
			return nil, state, err
		}

		return nil, state, repo.ErrNoQuestions
	}

	normalOrder := BuildDifficultyOrderFromLastResult(
		state,
		lastQuestionDifficulty,
		lastAnswerCorrect,
		minDifficulty,
		maxDifficulty,
	)

	question, err := s.questions.FindNextGlobalQuestionByDifficulties(
		ctx,
		attemptID,
		subjectID,
		normalOrder,
	)
	if err == nil {
		return &selectedQuestion{
			Question: question,
			State:    state,
		}, state, nil
	}

	if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
		return nil, state, err
	}

	expandedOrder := BuildExpandedDifficultyOrderFromLastResult(
		state,
		lastQuestionDifficulty,
		lastAnswerCorrect,
		minDifficulty,
		maxDifficulty,
	)

	question, err = s.questions.FindNextGlobalQuestionByDifficulties(
		ctx,
		attemptID,
		subjectID,
		expandedOrder,
	)
	if err == nil {
		return &selectedQuestion{
			Question: question,
			State:    state,
		}, state, nil
	}

	if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
		return nil, state, err
	}

	state.IsLocked = true

	if err = s.attempts.UpdateAttemptState(ctx, stateToUpdateInput(state)); err != nil {
		return nil, state, err
	}

	return nil, state, repo.ErrNoQuestions
}

func allStatesLocked(states []models.AttemptSubtopicState) bool {
	if len(states) == 0 {
		return true
	}

	for _, state := range states {
		if !state.IsLocked && state.AskedCount < state.MaxItems {
			return false
		}
	}

	return true
}
