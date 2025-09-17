package utils

import "math"

func RoundWithTwoDecimalPlace(f float64) float64 {
	if f == 0 {
		return 0
	}

	return math.Round(f*100) / 100
}
