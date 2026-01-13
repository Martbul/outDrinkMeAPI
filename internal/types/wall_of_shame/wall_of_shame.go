package wallofshame

import "time"

type WallOfShameItem struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"userId" db:"user_id"` 
	Text      string    `json:"text" db:"text"`
	Tier      string    `json:"tier" db:"tier"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

type CreateShameRequest struct {
	Text string `json:"text" validate:"required"`
	Tier string `json:"tier" validate:"required,oneof=S A B C D F"`
}