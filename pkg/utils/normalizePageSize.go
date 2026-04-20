package utils

func NormalizePageSize(v int32, def int32, max int32) int32 {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}
