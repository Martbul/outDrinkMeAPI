package calendar

import "time"

type CalendarDay struct {
	Date       time.Time `json:"date" db:"date"`
	DrankToday bool      `json:"drank_today" db:"drank_today"`
	IsToday    bool      `json:"is_today"`
}

type CalendarResponse struct {
	Year  int            `json:"year"`
	Month int            `json:"month"`
	Days  []*CalendarDay `json:"days"`
}