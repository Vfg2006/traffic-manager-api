package utils

import "time"

func ParseDate(dateStr string) (*time.Time, error) {
	var date time.Time

	if dateStr != "" {
		incomingDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, err
		}

		date = incomingDate
	}

	return &date, nil
}
