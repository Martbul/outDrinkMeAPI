package subscription

import "time"

type Subscription struct {
	ID                   string    `json:"id" db:"id"`
	UserID               string    `json:"userId" db:"user_id"`
	StripeCustomerID     string    `json:"stripeCustomerId" db:"stripe_customer_id"`
	StripeSubscriptionID string    `json:"stripeSubscriptionId" db:"stripe_subscription_id"`
	StripePriceID        string    `json:"stripePriceId" db:"stripe_price_id"`
	Status               string    `json:"status" db:"status"`
	CurrentPeriodEnd     time.Time `json:"currentPeriodEnd" db:"current_period_end"`
	CreatedAt            time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt            time.Time `json:"updatedAt" db:"updated_at"`
}

type SubscribeRequest struct {
	PriceID string `json:"priceId"` 
}

type SubscribeResponse struct {
	CheckoutURL string `json:"checkoutUrl"`
}