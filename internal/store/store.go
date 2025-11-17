package store

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Slug         string    `json:"slug" db:"slug"`
	Description  *string   `json:"description" db:"description"`
	Icon         *string   `json:"icon" db:"icon"`
	DisplayOrder int       `json:"display_order" db:"display_order"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Item struct {
	ID          uuid.UUID `json:"id" db:"id"`
	CategoryID  uuid.UUID `json:"category_id" db:"category_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	ItemType    string    `json:"item_type" db:"item_type"`
	ImageURL    string    `json:"image_url" db:"image_url"`
	BasePrice   int       `json:"base_price" db:"base_price"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type ItemWithDeal struct {
	Item
	HasDeal            bool       `json:"has_deal"`
	DiscountPercentage *int       `json:"discount_percentage,omitempty"`
	DiscountedPrice    *int       `json:"discounted_price,omitempty"`
	DealEndDate        *time.Time `json:"deal_end_date,omitempty"`
}

type Deal struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	ItemID             uuid.UUID `json:"item_id" db:"item_id"`
	DiscountPercentage int       `json:"discount_percentage" db:"discount_percentage"`
	DiscountedPrice    int       `json:"discounted_price" db:"discounted_price"`
	StartDate          time.Time `json:"start_date" db:"start_date"`
	EndDate            time.Time `json:"end_date" db:"end_date"`
	IsActive           bool      `json:"is_active" db:"is_active"`
	DealType           *string   `json:"deal_type" db:"deal_type"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

type Purchase struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	ItemID        *uuid.UUID `json:"item_id" db:"item_id"`
	PurchaseType  string     `json:"purchase_type" db:"purchase_type"`
	AmountPaid    *int       `json:"amount_paid" db:"amount_paid"`
	Currency      *string    `json:"currency" db:"currency"`
	TransactionID *string    `json:"transaction_id" db:"transaction_id"`
	Status        string     `json:"status" db:"status"`
	PurchasedAt   time.Time  `json:"purchased_at" db:"purchased_at"`
}

type InventoryItem struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	ItemID     uuid.UUID  `json:"item_id" db:"item_id"`
	Quantity   int        `json:"quantity" db:"quantity"`
	IsEquipped bool       `json:"is_equipped" db:"is_equipped"`
	AcquiredAt time.Time  `json:"acquired_at" db:"acquired_at"`
	ExpiresAt  *time.Time `json:"expires_at" db:"expires_at"`
}

type EquippedItem struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	ItemType   string    `json:"item_type" db:"item_type"`
	ItemID     uuid.UUID `json:"item_id" db:"item_id"`
	EquippedAt time.Time `json:"equipped_at" db:"equipped_at"`
}

type PurchaseItemRequest struct {
	ItemID string `json:"item_id" validate:"required"`
}

type PurchaseGemsRequest struct {
	GemsID string `json:"gems_id" validate:"required"`
}


type EquipItemRequest struct {
	ItemID string `json:"item_id" validate:"required"`
}

type StoreResponse struct {
	Categories      []*Category     `json:"categories"`
	RegularDeals    []*ItemWithDeal `json:"regular_deals"`
	ProDeals        []*ItemWithDeal `json:"pro_deals"`
	Flags           []*ItemWithDeal `json:"flags"`
	SmokingDevices  []*ItemWithDeal `json:"smoking_devices"`
	Themes          []*ItemWithDeal `json:"themes"`
	GemPacks        []*ItemWithDeal `json:"gem_packs"`
	UserGemsBalance int             `json:"user_gems_balance"`
}
