package premium

import (
	"time"
)

type Premium struct {
	ID             int       `json:"id" db:"id"`
	UserID         string    `json:"userId" db:"user_id"`
	Username       string    `json:"username" db:"username"`
	UserImageURL   string    `json:"userImageUrl" db:"user_image_url"`
	QRCodeData     string    `json:"qrCodeData" db:"qr_code_data"`
	VenuesVisited  int       `json:"venuesVisited" db:"venues_visited"`
	ValidUntil     time.Time `json:"validUntil" db:"valid_until"`
	IsActive       bool      `json:"isActive" db:"is_active"`
	SubscriptionID string    `json:"subscriptionId" db:"subscription_id"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time `json:"updatedAt" db:"updated_at"`
}
