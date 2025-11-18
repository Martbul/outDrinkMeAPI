package user

import "time"

type User struct {
	ID                   string    `json:"id"`
	ClerkID              string    `json:"clerkId"`
	Email                string    `json:"email"`
	Username             string    `json:"username"`
	FirstName            string    `json:"firstName"`
	LastName             string    `json:"lastName"`
	ImageURL             string    `json:"imageUrl,omitempty"`
	EmailVerified        bool      `json:"emailVerified"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
	Gems                 int       `json:"gems"`
	XP                   int       `json:"xp"`
	AllDaysDrinkingCount int       `json:"all_days_drinking_count"`
}


type DrunkThought struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	UserImageURL string    `json:"user_image_url"`
	Thought      string    `json:"thought"`
	CreatedAt    time.Time `json:"created_at"`
}
