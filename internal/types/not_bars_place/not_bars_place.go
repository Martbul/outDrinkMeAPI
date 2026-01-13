package notbarsplace

import (
	"time"
)

type NotBarsPlace struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"userId" db:"user_id"` 
	Name      string    `json:"name" db:"name"`
	Visited   bool      `json:"visited" db:"visited"`
	Rating    int       `json:"rating" db:"rating"`    
	IsCustom  bool      `json:"isCustom" db:"is_custom"`
	SortOrder int       `json:"sortOrder" db:"sort_order"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}