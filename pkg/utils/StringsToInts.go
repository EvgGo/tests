package utils

import (
	"fmt"
	"strconv"
)

func StringsToInts(values []string) ([]int, error) {
	result := make([]int, 0, len(values))

	for _, v := range values {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %q to int: %w", v, err)
		}
		result = append(result, n)
	}

	return result, nil
}
