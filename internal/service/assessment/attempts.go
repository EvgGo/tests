package assessmentsvc

import (
	"context"
	"errors"
	"testing/internal/models"
	"testing/internal/repo"
	"time"
)

func (s *Service) StartAttempt(
	ctx context.Context,
	input StartAttemptInput,
) (*StartAttemptResult, error) {
	input.UserID = normalizeUserID(input.UserID)

	if input.UserID == "" || input.AssessmentID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	var result *StartAttemptResult

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		existingAttempt, err := s.attempts.FindInProgressAttempt(
			txCtx,
			input.UserID,
			input.AssessmentID,
		)
		if err != nil && !errors.Is(err, repo.ErrNotFound) {
			return err
		}

		if existingAttempt != nil {
			if !input.RestartIfInProgress {
				return repo.ErrAlreadyExists
			}

			lockedExistingAttempt, err := s.attempts.GetAttemptForUpdate(
				txCtx,
				existingAttempt.ID,
				input.UserID,
			)
			if err != nil {
				return err
			}

			if lockedExistingAttempt.Status == models.AttemptStatusInProgress {
				states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, lockedExistingAttempt.ID)
				if err != nil {
					return err
				}

				overall := RecalculateOverall(states)

				_, err = s.attempts.FinishAttempt(txCtx, models.FinishAttemptInput{
					AttemptID:         lockedExistingAttempt.ID,
					Status:            models.AttemptStatusAbandoned,
					FinishReason:      models.FinishReasonSystem,
					CompletedAt:       now,
					OverallLevel:      overall.OverallLevel,
					OverallLevelScore: overall.OverallLevelScore,
					OverallConfidence: overall.OverallConfidence,
				})
				if err != nil {
					return err
				}
			}
		}

		assessment, err := s.catalog.GetAssessmentForStart(txCtx, input.AssessmentID)
		if err != nil {
			return err
		}

		startedAt := now
		expiresAt := startedAt.Add(time.Duration(assessment.DurationSeconds) * time.Second)

		attempt, err := s.attempts.CreateAttempt(txCtx, models.CreateAttemptInput{
			AssessmentID:    assessment.ID,
			UserID:          input.UserID,
			StartedAt:       startedAt,
			ExpiresAt:       expiresAt,
			DurationSeconds: assessment.DurationSeconds,
		})
		if err != nil {
			return err
		}

		// GLOBAL MODE:
		// создаем только одну carrier-state запись.
		if assessment.Mode == models.AssessmentModeGlobal {
			if len(assessment.Subtopics) == 0 {
				return repo.ErrInvalidState
			}

			// Берем первую assessment_subtopic как carrier-state.
			// Вопросы при этом будут выбираться по всему subject_id.
			carrier := buildEffectiveSubtopicConfig(*assessment, assessment.Subtopics[0])

			err = s.attempts.CreateAttemptState(txCtx, models.CreateAttemptStateInput{
				AttemptID:       attempt.ID,
				SubtopicID:      carrier.SubtopicID,
				LevelScore:      float64(assessment.StartDifficulty),
				EstimatedLevel:  assessment.StartDifficulty,
				Confidence:      0.000,
				Weight:          1.0,
				Priority:        0,
				MinItems:        assessment.MinItems,
				MaxItems:        assessment.MaxItems,
				StartDifficulty: assessment.StartDifficulty,
				StopConfidence:  assessment.StopConfidence,
			})
			if err != nil {
				return err
			}

			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}
			if len(states) != 1 {
				return repo.ErrInvalidState
			}

			selected, updatedState, err := s.pickNextGlobalQuestionLocked(
				txCtx,
				attempt.ID,
				assessment.SubjectID,
				states[0],
				assessment.MinDifficulty,
				assessment.MaxDifficulty,
			)
			if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
				return err
			}

			if errors.Is(err, repo.ErrNoQuestions) || selected == nil {
				overall := RecalculateOverall([]models.AttemptSubtopicState{updatedState})
				applyOverallToAttempt(attempt, overall)

				finishedAttempt, err := s.attempts.FinishAttempt(txCtx, models.FinishAttemptInput{
					AttemptID:         attempt.ID,
					Status:            models.AttemptStatusCompleted,
					FinishReason:      models.FinishReasonSystem,
					CompletedAt:       now,
					OverallLevel:      overall.OverallLevel,
					OverallLevelScore: overall.OverallLevelScore,
					OverallConfidence: overall.OverallConfidence,
				})
				if err != nil {
					return err
				}

				result = &StartAttemptResult{
					Attempt:       *finishedAttempt,
					FirstQuestion: nil,
					Progress:      buildProgress(*finishedAttempt, now),
				}

				return nil
			}

			result = &StartAttemptResult{
				Attempt:       *attempt,
				FirstQuestion: selected.Question,
				Progress:      buildProgress(*attempt, now),
			}

			return nil
		}

		// SUBTOPIC MODE:
		for _, config := range assessment.Subtopics {
			effective := buildEffectiveSubtopicConfig(*assessment, config)

			err = s.attempts.CreateAttemptState(txCtx, models.CreateAttemptStateInput{
				AttemptID:       attempt.ID,
				SubtopicID:      effective.SubtopicID,
				LevelScore:      float64(effective.StartDifficulty),
				EstimatedLevel:  effective.StartDifficulty,
				Confidence:      0.000,
				Weight:          effective.Weight,
				Priority:        effective.Priority,
				MinItems:        effective.MinItems,
				MaxItems:        effective.MaxItems,
				StartDifficulty: effective.StartDifficulty,
				StopConfidence:  effective.StopConfidence,
			})
			if err != nil {
				return err
			}
		}

		states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
		if err != nil {
			return err
		}

		selected, states, err := s.pickNextQuestionLocked(
			txCtx,
			attempt.ID,
			states,
			assessment.MinDifficulty,
			assessment.MaxDifficulty,
		)
		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return err
		}

		if errors.Is(err, repo.ErrNoQuestions) || selected == nil {
			overall := RecalculateOverall(states)
			applyOverallToAttempt(attempt, overall)

			finishedAttempt, err := s.attempts.FinishAttempt(txCtx, models.FinishAttemptInput{
				AttemptID:         attempt.ID,
				Status:            models.AttemptStatusCompleted,
				FinishReason:      models.FinishReasonSystem,
				CompletedAt:       now,
				OverallLevel:      overall.OverallLevel,
				OverallLevelScore: overall.OverallLevelScore,
				OverallConfidence: overall.OverallConfidence,
			})
			if err != nil {
				return err
			}

			result = &StartAttemptResult{
				Attempt:       *finishedAttempt,
				FirstQuestion: nil,
				Progress:      buildProgress(*finishedAttempt, now),
			}

			return nil
		}

		result = &StartAttemptResult{
			Attempt:       *attempt,
			FirstQuestion: selected.Question,
			Progress:      buildProgress(*attempt, now),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) GetAttempt(
	ctx context.Context,
	userID string,
	attemptID int64,
	includeSubtopicStates bool,
) (*models.Attempt, error) {

	userID = normalizeUserID(userID)

	if userID == "" || attemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	var result *models.Attempt

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		attempt, err := s.attempts.GetAttemptForUpdate(txCtx, attemptID, userID)
		if err != nil {
			return err
		}

		if isAttemptExpired(*attempt, now) {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}

			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusExpired,
				models.FinishReasonTimeout,
				now,
			)
			if err != nil {
				return err
			}
		}

		if includeSubtopicStates {
			states, err := s.attempts.ListAttemptStates(txCtx, attempt.ID)
			if err != nil {
				return err
			}

			attempt.SubtopicStates = states
		}

		result = attempt

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) GetAttemptProgress(
	ctx context.Context,
	userID string,
	attemptID int64,
) (*models.AttemptProgress, error) {

	userID = normalizeUserID(userID)

	if userID == "" || attemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	var result *models.AttemptProgress

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		attempt, err := s.attempts.GetAttemptForUpdate(txCtx, attemptID, userID)
		if err != nil {
			return err
		}

		if isAttemptExpired(*attempt, now) {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}

			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusExpired,
				models.FinishReasonTimeout,
				now,
			)
			if err != nil {
				return err
			}
		}

		progress := buildProgress(*attempt, now)
		result = &progress

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) GetNextQuestion(
	ctx context.Context,
	input GetNextQuestionInput,
) (*GetNextQuestionResult, error) {

	input.UserID = normalizeUserID(input.UserID)

	if input.UserID == "" || input.AttemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	var result *GetNextQuestionResult

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		attempt, err := s.attempts.GetAttemptForUpdate(txCtx, input.AttemptID, input.UserID)
		if err != nil {
			return err
		}

		if isTerminalAttemptStatus(attempt.Status) {
			result = &GetNextQuestionResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		if isAttemptExpired(*attempt, now) {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}

			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusExpired,
				models.FinishReasonTimeout,
				now,
			)
			if err != nil {
				return err
			}

			result = &GetNextQuestionResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		assessment, err := s.catalog.GetAssessment(txCtx, attempt.AssessmentID, false)
		if err != nil {
			return err
		}

		// GLOBAL MODE
		if assessment.Mode == models.AssessmentModeGlobal {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}
			if len(states) != 1 {
				return repo.ErrInvalidState
			}

			selected, updatedState, err := s.pickNextGlobalQuestionLocked(
				txCtx,
				attempt.ID,
				assessment.SubjectID,
				states[0],
				assessment.MinDifficulty,
				assessment.MaxDifficulty,
			)
			if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
				return err
			}

			if errors.Is(err, repo.ErrNoQuestions) || selected == nil {
				overall := RecalculateOverall([]models.AttemptSubtopicState{updatedState})
				applyOverallToAttempt(attempt, overall)

				attempt, err = s.finishAttemptLocked(
					txCtx,
					*attempt,
					[]models.AttemptSubtopicState{updatedState},
					models.AttemptStatusCompleted,
					models.FinishReasonSystem,
					now,
				)
				if err != nil {
					return err
				}

				result = &GetNextQuestionResult{
					Completed:    true,
					Progress:     buildProgress(*attempt, now),
					NextQuestion: nil,
				}
				return nil
			}

			result = &GetNextQuestionResult{
				Completed:    false,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: selected.Question,
			}
			return nil
		}

		// SUBTOPIC MODE
		states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
		if err != nil {
			return err
		}

		selected, states, err := s.pickNextQuestionLocked(
			txCtx,
			attempt.ID,
			states,
			assessment.MinDifficulty,
			assessment.MaxDifficulty,
		)
		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return err
		}

		if errors.Is(err, repo.ErrNoQuestions) || selected == nil || allStatesLocked(states) {
			overall := RecalculateOverall(states)
			applyOverallToAttempt(attempt, overall)

			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusCompleted,
				models.FinishReasonSystem,
				now,
			)
			if err != nil {
				return err
			}

			result = &GetNextQuestionResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		result = &GetNextQuestionResult{
			Completed:    false,
			Progress:     buildProgress(*attempt, now),
			NextQuestion: selected.Question,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) SubmitAnswer(
	ctx context.Context,
	input SubmitAnswerInput,
) (*SubmitAnswerResult, error) {
	input.UserID = normalizeUserID(input.UserID)

	if input.UserID == "" ||
		input.AttemptID <= 0 ||
		input.QuestionID <= 0 ||
		input.SelectedOptionID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	var result *SubmitAnswerResult

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		attempt, err := s.attempts.GetAttemptForUpdate(txCtx, input.AttemptID, input.UserID)
		if err != nil {
			return err
		}

		if attempt.Status != models.AttemptStatusInProgress {
			return repo.ErrInvalidState
		}

		if isAttemptExpired(*attempt, now) {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}

			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusExpired,
				models.FinishReasonTimeout,
				now,
			)
			if err != nil {
				return err
			}

			result = &SubmitAnswerResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		assessment, err := s.catalog.GetAssessment(txCtx, attempt.AssessmentID, false)
		if err != nil {
			return err
		}

		// GLOBAL MODE
		if assessment.Mode == models.AssessmentModeGlobal {
			states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
			if err != nil {
				return err
			}
			if len(states) != 1 {
				return repo.ErrInvalidState
			}

			currentState := states[0]

			selected, _, err := s.pickNextGlobalQuestionLocked(
				txCtx,
				attempt.ID,
				assessment.SubjectID,
				currentState,
				assessment.MinDifficulty,
				assessment.MaxDifficulty,
			)
			if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
				return err
			}

			if errors.Is(err, repo.ErrNoQuestions) || selected == nil {
				attempt, err = s.finishAttemptLocked(
					txCtx,
					*attempt,
					[]models.AttemptSubtopicState{currentState},
					models.AttemptStatusCompleted,
					models.FinishReasonSystem,
					now,
				)
				if err != nil {
					return err
				}

				result = &SubmitAnswerResult{
					Completed:    true,
					Progress:     buildProgress(*attempt, now),
					NextQuestion: nil,
				}
				return nil
			}

			if selected.Question.ID != input.QuestionID {
				return repo.ErrQuestionMismatch
			}

			question, err := s.questions.GetQuestionForAnswer(txCtx, input.QuestionID)
			if err != nil {
				return err
			}

			if question.CorrectOptionID == nil {
				return repo.ErrInvalidState
			}

			optionBelongs, err := s.questions.OptionBelongsToQuestion(
				txCtx,
				input.QuestionID,
				input.SelectedOptionID,
			)
			if err != nil {
				return err
			}

			if !optionBelongs {
				return repo.ErrOptionMismatch
			}

			isCorrect := input.SelectedOptionID == *question.CorrectOptionID
			seqNo := attempt.SequenceNo + 1

			err = s.attempts.CreateAttemptAnswer(txCtx, models.CreateAttemptAnswerInput{
				AttemptID:        attempt.ID,
				SeqNo:            seqNo,
				QuestionID:       question.ID,
				SubtopicID:       question.SubtopicID,
				Difficulty:       question.Difficulty,
				SelectedOptionID: input.SelectedOptionID,
				IsCorrect:        isCorrect,
			})
			if err != nil {
				return err
			}

			updateResult, err := ApplyAnswerUpdate(AnswerUpdateInput{
				State:              currentState,
				QuestionDifficulty: question.Difficulty,
				IsCorrect:          isCorrect,
				MinDifficulty:      assessment.MinDifficulty,
				MaxDifficulty:      assessment.MaxDifficulty,
			})
			if err != nil {
				return err
			}

			nextState := updateResult.State
			nextState.LastQuestionID = &question.ID

			err = s.attempts.UpdateAttemptState(txCtx, stateToUpdateInput(nextState))
			if err != nil {
				return err
			}

			overall := RecalculateOverall([]models.AttemptSubtopicState{nextState})

			err = s.attempts.UpdateAttemptAggregate(txCtx, models.UpdateAttemptAggregateInput{
				AttemptID:         attempt.ID,
				SequenceNo:        seqNo,
				OverallLevel:      overall.OverallLevel,
				OverallLevelScore: overall.OverallLevelScore,
				OverallConfidence: overall.OverallConfidence,
			})
			if err != nil {
				return err
			}

			attempt.SequenceNo = seqNo
			applyOverallToAttempt(attempt, overall)

			nextSelected, updatedState, err := s.pickNextGlobalQuestionAfterAnswerLocked(
				txCtx,
				attempt.ID,
				assessment.SubjectID,
				nextState,
				assessment.MinDifficulty,
				assessment.MaxDifficulty,
				question.Difficulty,
				isCorrect,
			)
			if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
				return err
			}

			if errors.Is(err, repo.ErrNoQuestions) || nextSelected == nil {
				attempt, err = s.finishAttemptLocked(
					txCtx,
					*attempt,
					[]models.AttemptSubtopicState{updatedState},
					models.AttemptStatusCompleted,
					models.FinishReasonSystem,
					now,
				)
				if err != nil {
					return err
				}

				result = &SubmitAnswerResult{
					Completed:    true,
					Progress:     buildProgress(*attempt, now),
					NextQuestion: nil,
				}
				return nil
			}

			result = &SubmitAnswerResult{
				Completed:    false,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nextSelected.Question,
			}
			return nil
		}

		// SUBTOPIC MODE
		states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
		if err != nil {
			return err
		}

		selected, states, err := s.pickNextQuestionLocked(
			txCtx,
			attempt.ID,
			states,
			assessment.MinDifficulty,
			assessment.MaxDifficulty,
		)
		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return err
		}

		if errors.Is(err, repo.ErrNoQuestions) || selected == nil {
			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusCompleted,
				models.FinishReasonSystem,
				now,
			)
			if err != nil {
				return err
			}

			result = &SubmitAnswerResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		if selected.Question.ID != input.QuestionID {
			return repo.ErrQuestionMismatch
		}

		question, err := s.questions.GetQuestionForAnswer(txCtx, input.QuestionID)
		if err != nil {
			return err
		}

		if question.CorrectOptionID == nil {
			return repo.ErrInvalidState
		}

		optionBelongs, err := s.questions.OptionBelongsToQuestion(
			txCtx,
			input.QuestionID,
			input.SelectedOptionID,
		)
		if err != nil {
			return err
		}

		if !optionBelongs {
			return repo.ErrOptionMismatch
		}

		isCorrect := input.SelectedOptionID == *question.CorrectOptionID
		seqNo := attempt.SequenceNo + 1

		err = s.attempts.CreateAttemptAnswer(txCtx, models.CreateAttemptAnswerInput{
			AttemptID:        attempt.ID,
			SeqNo:            seqNo,
			QuestionID:       question.ID,
			SubtopicID:       question.SubtopicID,
			Difficulty:       question.Difficulty,
			SelectedOptionID: input.SelectedOptionID,
			IsCorrect:        isCorrect,
		})
		if err != nil {
			return err
		}

		updateResult, err := ApplyAnswerUpdate(AnswerUpdateInput{
			State:              selected.State,
			QuestionDifficulty: question.Difficulty,
			IsCorrect:          isCorrect,
			MinDifficulty:      assessment.MinDifficulty,
			MaxDifficulty:      assessment.MaxDifficulty,
		})
		if err != nil {
			return err
		}

		nextState := updateResult.State
		nextState.LastQuestionID = &question.ID

		err = s.attempts.UpdateAttemptState(txCtx, stateToUpdateInput(nextState))
		if err != nil {
			return err
		}

		states = replaceState(states, nextState)

		overall := RecalculateOverall(states)

		err = s.attempts.UpdateAttemptAggregate(txCtx, models.UpdateAttemptAggregateInput{
			AttemptID:         attempt.ID,
			SequenceNo:        seqNo,
			OverallLevel:      overall.OverallLevel,
			OverallLevelScore: overall.OverallLevelScore,
			OverallConfidence: overall.OverallConfidence,
		})
		if err != nil {
			return err
		}

		attempt.SequenceNo = seqNo
		applyOverallToAttempt(attempt, overall)

		nextSelected, states, err := s.pickNextQuestionAfterAnswerLocked(
			txCtx,
			attempt.ID,
			states,
			assessment.MinDifficulty,
			assessment.MaxDifficulty,
			question.SubtopicID,
			question.Difficulty,
			isCorrect,
		)
		if err != nil && !errors.Is(err, repo.ErrNoQuestions) {
			return err
		}

		if errors.Is(err, repo.ErrNoQuestions) || nextSelected == nil || allStatesLocked(states) {
			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusCompleted,
				models.FinishReasonSystem,
				now,
			)
			if err != nil {
				return err
			}

			result = &SubmitAnswerResult{
				Completed:    true,
				Progress:     buildProgress(*attempt, now),
				NextQuestion: nil,
			}
			return nil
		}

		result = &SubmitAnswerResult{
			Completed:    false,
			Progress:     buildProgress(*attempt, now),
			NextQuestion: nextSelected.Question,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) FinishAttempt(
	ctx context.Context,
	input FinishAttemptInput,
) (*models.Attempt, error) {

	input.UserID = normalizeUserID(input.UserID)

	if input.UserID == "" || input.AttemptID <= 0 {
		return nil, repo.ErrInvalidInput
	}

	if input.Action != models.FinishActionComplete && input.Action != models.FinishActionAbandon {
		return nil, repo.ErrInvalidInput
	}

	var result *models.Attempt

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		now := s.clock().UTC()

		attempt, err := s.attempts.GetAttemptForUpdate(txCtx, input.AttemptID, input.UserID)
		if err != nil {
			return err
		}

		if isTerminalAttemptStatus(attempt.Status) {
			result = attempt
			return nil
		}

		states, err := s.attempts.ListAttemptStatesForUpdate(txCtx, attempt.ID)
		if err != nil {
			return err
		}

		if isAttemptExpired(*attempt, now) {
			attempt, err = s.finishAttemptLocked(
				txCtx,
				*attempt,
				states,
				models.AttemptStatusExpired,
				models.FinishReasonTimeout,
				now,
			)
			if err != nil {
				return err
			}

			result = attempt
			return nil
		}

		status := models.AttemptStatusCompleted
		reason := models.FinishReasonUserFinish

		if input.Action == models.FinishActionAbandon {
			status = models.AttemptStatusAbandoned
			reason = models.FinishReasonUserAbandon
		}

		attempt, err = s.finishAttemptLocked(
			txCtx,
			*attempt,
			states,
			status,
			reason,
			now,
		)
		if err != nil {
			return err
		}

		result = attempt

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) finishAttemptLocked(
	ctx context.Context,
	attempt models.Attempt,
	states []models.AttemptSubtopicState,
	status models.AttemptStatus,
	reason models.FinishReason,
	now time.Time,
) (*models.Attempt, error) {

	overall := RecalculateOverall(states)

	finishedAttempt, err := s.attempts.FinishAttempt(ctx, models.FinishAttemptInput{
		AttemptID:         attempt.ID,
		Status:            status,
		FinishReason:      reason,
		CompletedAt:       now,
		OverallLevel:      overall.OverallLevel,
		OverallLevelScore: overall.OverallLevelScore,
		OverallConfidence: overall.OverallConfidence,
	})
	if err != nil {
		return nil, err
	}

	return finishedAttempt, nil
}

func buildEffectiveSubtopicConfig(
	assessment models.Assessment,
	config models.AssessmentSubtopicConfig,
) models.EffectiveAssessmentSubtopic {

	minItems := assessment.MinItems
	maxItems := assessment.MaxItems
	startDifficulty := assessment.StartDifficulty
	stopConfidence := assessment.StopConfidence

	if config.MinItems != nil {
		minItems = *config.MinItems
	}

	if config.MaxItems != nil {
		maxItems = *config.MaxItems
	}

	if config.StartDifficulty != nil {
		startDifficulty = *config.StartDifficulty
	}

	if config.StopConfidence != nil {
		stopConfidence = *config.StopConfidence
	}

	if minItems < 0 {
		minItems = 0
	}

	if maxItems < 1 {
		maxItems = 1
	}

	if maxItems < minItems {
		maxItems = minItems
	}

	if startDifficulty < assessment.MinDifficulty {
		startDifficulty = assessment.MinDifficulty
	}

	if startDifficulty > assessment.MaxDifficulty {
		startDifficulty = assessment.MaxDifficulty
	}

	if stopConfidence < 0 {
		stopConfidence = 0
	}

	if stopConfidence > 1 {
		stopConfidence = 1
	}

	return models.EffectiveAssessmentSubtopic{
		AssessmentID:    assessment.ID,
		SubtopicID:      config.SubtopicID,
		Weight:          config.Weight,
		Priority:        config.Priority,
		MinItems:        minItems,
		MaxItems:        maxItems,
		StartDifficulty: startDifficulty,
		StopConfidence:  stopConfidence,
	}
}
