package formatter

import "math"

// roundAvg returns the mean of sum over count, rounded to one decimal place, or
// 0 when count is zero. It is the single rounding rule shared by every
// per-section average across the text and JSON formatters.
func roundAvg(sum float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return math.Round(sum/float64(count)*10) / 10
}
