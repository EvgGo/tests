package postgres

import (
	"strconv"
	"strings"
	"testing/internal/repo"
)

func normalizePageSize(pageSize int, defaultValue int, maxValue int) int {
	if pageSize <= 0 {
		return defaultValue
	}
	if pageSize > maxValue {
		return maxValue
	}

	return pageSize
}

func decodeIDCursor(pageToken string) (int64, error) {
	pageToken = strings.TrimSpace(pageToken)
	if pageToken == "" {
		return 0, nil
	}

	value, err := strconv.ParseInt(pageToken, 10, 64)
	if err != nil || value < 0 {
		return 0, repo.ErrInvalidInput
	}

	return value, nil
}

func encodeIDCursor(id int64) string {
	if id <= 0 {
		return ""
	}

	return strconv.FormatInt(id, 10)
}
