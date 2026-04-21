package assessmentsvc

import (
	"math"
	"testing/internal/models"
	"testing/internal/repo"
)

//1. ищем активную попытку
//2. если есть и restart_if_in_progress=false:
//вернуть AlreadyExists
//
//3. если есть и restart_if_in_progress=true:
//finish старую:
//status = abandoned
//finish_reason = system
//
//4. создаем новую attempt
//5. создаем attempt_subtopic_state по всем assessment_subtopics
//6. выбираем первый вопрос

type AnswerUpdateInput struct {
	State              models.AttemptSubtopicState
	QuestionDifficulty int
	IsCorrect          bool
	MinDifficulty      int
	MaxDifficulty      int
}

type AnswerUpdateResult struct {
	State        models.AttemptSubtopicState
	Gap          int
	Step         float64
	LevelChanged bool
}

type OverallResult struct {
	OverallLevel      int
	OverallLevelScore float64
	OverallConfidence float64
}

func ApplyAnswerUpdate(input AnswerUpdateInput) (AnswerUpdateResult, error) {
	state := input.State

	if input.QuestionDifficulty < input.MinDifficulty || input.QuestionDifficulty > input.MaxDifficulty {
		return AnswerUpdateResult{}, repo.ErrInvalidInput
	}
	if state.EstimatedLevel < input.MinDifficulty || state.EstimatedLevel > input.MaxDifficulty {
		return AnswerUpdateResult{}, repo.ErrInvalidState
	}
	if state.LevelScore < float64(input.MinDifficulty) || state.LevelScore > float64(input.MaxDifficulty) {
		return AnswerUpdateResult{}, repo.ErrInvalidState
	}

	askedCountBefore := state.AskedCount
	prevLevel := state.EstimatedLevel
	prevScore := state.LevelScore
	prevConfidence := state.Confidence

	gap := input.QuestionDifficulty - prevLevel

	step := calculateStep(input.IsCorrect, gap, state.ConsecutiveCorrect, state.ConsecutiveWrong)
	nextScore := clampFloat(prevScore+step, float64(input.MinDifficulty), float64(input.MaxDifficulty))
	nextLevel := roundLevel(nextScore, input.MinDifficulty, input.MaxDifficulty)

	confGain := calculateConfidenceGain(askedCountBefore, gap)

	levelChanged := nextLevel != prevLevel
	nextConfidence := calculateNextConfidence(prevConfidence, confGain, levelChanged)

	state.LevelScore = roundFloat(nextScore, 2)
	state.EstimatedLevel = nextLevel
	state.Confidence = roundFloat(nextConfidence, 3)

	if input.IsCorrect {
		state.AskedCount++
		state.CorrectCount++
		state.ConsecutiveCorrect++
		state.ConsecutiveWrong = 0
	} else {
		state.AskedCount++
		state.WrongCount++
		state.ConsecutiveWrong++
		state.ConsecutiveCorrect = 0
	}

	state.LastAnswerCorrect = boolPtr(input.IsCorrect)

	if ShouldLockSubtopic(state) {
		state.IsLocked = true
	}

	return AnswerUpdateResult{
		State:        state,
		Gap:          gap,
		Step:         step,
		LevelChanged: levelChanged,
	}, nil
}

func calculateStep(isCorrect bool, gap int, consecutiveCorrect int, consecutiveWrong int) float64 {

	if isCorrect {
		step := 0.0

		switch {
		case gap >= 1:
			// Правильный ответ на более сложном вопросе хороший показатель
			step = 0.35
		case gap == 0:
			step = 0.24
		default:
			// Правильный ответ на более легком вопросе слабый показатель
			step = 0.12
		}

		// Усиление только за серию, а не за один ответ
		if consecutiveCorrect >= 1 {
			step += 0.05
		}
		if consecutiveCorrect >= 2 {
			step += 0.05
		}

		return step
	}

	step := 0.0

	switch {
	case gap <= -1:
		// Ошибка на более легком вопросе - сильный сигнал вниз,
		// но не настолько сильный, чтобы сразу ронять на 1 уровень
		step = -0.38
	case gap == 0:
		step = -0.26
	default:
		// Ошибка на более сложном вопросе - слабый негативный сигнал
		step = -0.14
	}

	// Серия ошибок усиливает снижение
	if consecutiveWrong >= 1 {
		step -= 0.06
	}
	if consecutiveWrong >= 2 {
		step -= 0.06
	}

	return step
}

func calculateConfidenceGain(askedCountBefore int, gap int) float64 {

	baseGain := 0.18 / (1 + 0.20*float64(askedCountBefore))

	infoAdjust := 0.0

	switch {
	case absInt(gap) == 0:
		infoAdjust = 0.04
	case absInt(gap) == 1:
		infoAdjust = 0.00
	default:
		infoAdjust = -0.03
	}

	return clampFloat(baseGain+infoAdjust, 0.05, 0.22)
}

func ShouldLockSubtopic(state models.AttemptSubtopicState) bool {
	if state.AskedCount >= state.MaxItems {
		return true
	}

	if state.AskedCount >= state.MinItems && state.Confidence >= state.StopConfidence {
		return true
	}

	return false
}

func BuildDifficultyOrder(state models.AttemptSubtopicState, minDifficulty int, maxDifficulty int) []int {

	scoreFloor := floorLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)
	scoreCeil := ceilLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)
	scoreNearest := nearestLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)

	var candidates []int

	// Первый вопрос по текущему состоянию:
	// идем максимально близко к score
	if state.LastAnswerCorrect == nil {
		candidates = []int{
			scoreNearest,
			scoreFloor,
			scoreCeil,
			scoreNearest - 1,
			scoreNearest + 1,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	if *state.LastAnswerCorrect {
		// После правильного ответа не прыгаем резко вверх:
		// сначала +1, потом текущий диапазон
		candidates = []int{
			scoreNearest + 1,
			scoreNearest,
			scoreFloor,
			scoreCeil,
			scoreNearest - 1,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	// После ошибки:
	// сначала -1, потом текущий, потом только ниже/выше
	candidates = []int{
		scoreNearest - 1,
		scoreNearest,
		scoreFloor,
		scoreCeil,
		scoreNearest - 2,
		scoreNearest + 1,
	}

	return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
}

func BuildExpandedDifficultyOrder(state models.AttemptSubtopicState, minDifficulty int, maxDifficulty int) []int {

	scoreFloor := floorLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)
	scoreCeil := ceilLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)
	scoreNearest := nearestLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)

	var candidates []int

	if state.LastAnswerCorrect == nil {
		candidates = []int{
			scoreNearest,
			scoreFloor,
			scoreCeil,
			scoreNearest - 1,
			scoreNearest + 1,
			scoreNearest - 2,
			scoreNearest + 2,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	if *state.LastAnswerCorrect {
		candidates = []int{
			scoreNearest + 1,
			scoreNearest,
			scoreFloor,
			scoreCeil,
			scoreNearest + 2,
			scoreNearest - 1,
			scoreNearest - 2,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	candidates = []int{
		scoreNearest - 1,
		scoreNearest,
		scoreFloor,
		scoreCeil,
		scoreNearest - 2,
		scoreNearest + 1,
		scoreNearest + 2,
	}

	return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
}

func calculateNextConfidence(prevConfidence float64, confGain float64, levelChanged bool) float64 {
	if !levelChanged {
		return math.Min(1.0, prevConfidence+confGain)
	}

	// Более мягкий сброс при смене уровня:
	// confidence не обнуляется, а лишь немного снижается
	return math.Min(1.0, math.Max(0.15, prevConfidence*0.85)+confGain*0.75)
}

func RecalculateOverall(states []models.AttemptSubtopicState) OverallResult {
	if len(states) == 0 {
		return OverallResult{}
	}

	totalWeight := 0.0
	scoreSum := 0.0
	confidenceSum := 0.0

	for _, state := range states {
		if state.Weight <= 0 {
			continue
		}

		if state.AskedCount <= 0 {
			continue
		}

		totalWeight += state.Weight
		scoreSum += state.LevelScore * state.Weight
		confidenceSum += state.Confidence * state.Weight
	}

	if totalWeight <= 0 {
		return OverallResult{}
	}

	overallScore := scoreSum / totalWeight
	overallConfidence := confidenceSum / totalWeight
	overallLevel := roundLevel(overallScore, 1, 5)

	return OverallResult{
		OverallLevel:      overallLevel,
		OverallLevelScore: roundFloat(overallScore, 2),
		OverallConfidence: roundFloat(overallConfidence, 3),
	}
}

func BuildDifficultyOrderFromLastResult(
	state models.AttemptSubtopicState,
	lastQuestionDifficulty int,
	lastAnswerCorrect bool,
	minDifficulty int,
	maxDifficulty int,
) []int {

	scoreNearest := nearestLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)

	var candidates []int

	if lastAnswerCorrect {
		// После успеха максимум на +1 вверх как приоритет
		base := maxInt(scoreNearest, lastQuestionDifficulty)

		candidates = []int{
			base + 1,
			base,
			base - 1,
			base + 2,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	// После ошибки не позволяем сразу проваливаться слишком низко,
	// если это не ошибка на совсем нижней границе
	switch lastQuestionDifficulty {
	case 1:
		candidates = []int{1, 2}
	case 2:
		candidates = []int{1, 2, 3}
	case 3:
		candidates = []int{2, 3, 1, 4}
	case 4:
		candidates = []int{3, 4, 2, 5}
	case 5:
		candidates = []int{4, 5, 3}
	default:
		candidates = []int{
			lastQuestionDifficulty - 1,
			lastQuestionDifficulty,
			lastQuestionDifficulty - 2,
			lastQuestionDifficulty + 1,
		}
	}

	return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
}

func BuildExpandedDifficultyOrderFromLastResult(
	state models.AttemptSubtopicState,
	lastQuestionDifficulty int,
	lastAnswerCorrect bool,
	minDifficulty int,
	maxDifficulty int,
) []int {

	scoreNearest := nearestLevelFromScore(state.LevelScore, minDifficulty, maxDifficulty)

	var candidates []int

	if lastAnswerCorrect {
		base := maxInt(scoreNearest, lastQuestionDifficulty)

		candidates = []int{
			base + 1,
			base,
			base - 1,
			base + 2,
			base - 2,
		}

		return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
	}

	switch lastQuestionDifficulty {
	case 1:
		candidates = []int{1, 2, 3}
	case 2:
		candidates = []int{1, 2, 3, 4}
	case 3:
		candidates = []int{2, 3, 1, 4, 5}
	case 4:
		candidates = []int{3, 4, 2, 5, 1}
	case 5:
		candidates = []int{4, 5, 3, 2}
	default:
		candidates = []int{
			lastQuestionDifficulty - 1,
			lastQuestionDifficulty,
			lastQuestionDifficulty - 2,
			lastQuestionDifficulty + 1,
			lastQuestionDifficulty + 2,
		}
	}

	return uniqueValidDifficulties(candidates, minDifficulty, maxDifficulty)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func roundLevel(score float64, minDifficulty int, maxDifficulty int) int {
	level := int(math.Floor(score + 0.5))
	return clampInt(level, minDifficulty, maxDifficulty)
}

func uniqueValidDifficulties(values []int, minDifficulty int, maxDifficulty int) []int {
	result := make([]int, 0, len(values))
	seen := make(map[int]bool, len(values))

	for _, value := range values {
		if value < minDifficulty || value > maxDifficulty {
			continue
		}
		if seen[value] {
			continue
		}

		seen[value] = true
		result = append(result, value)
	}

	return result
}

func floorLevelFromScore(score float64, minDifficulty int, maxDifficulty int) int {
	level := int(math.Floor(score))
	return clampInt(level, minDifficulty, maxDifficulty)
}

func ceilLevelFromScore(score float64, minDifficulty int, maxDifficulty int) int {
	level := int(math.Ceil(score))
	return clampInt(level, minDifficulty, maxDifficulty)
}

func nearestLevelFromScore(score float64, minDifficulty int, maxDifficulty int) int {
	level := int(math.Round(score))
	return clampInt(level, minDifficulty, maxDifficulty)
}

func clampFloat(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func roundFloat(value float64, precision int) float64 {
	multiplier := math.Pow10(precision)
	return math.Round(value*multiplier) / multiplier
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func boolPtr(value bool) *bool {
	return &value
}
