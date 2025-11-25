package clerk

import "encoding/json"

type ClerkWebhookEvent struct {
	Data   json.RawMessage `json:"data"`
	Object string          `json:"object"`
	Type   string          `json:"type"`
}



type ClerkUserData struct {
	ID                     string                    `json:"id"`
	EmailAddresses         []ClerkEmailAddress       `json:"email_addresses"`
	FirstName              string                    `json:"first_name"`
	LastName               string                    `json:"last_name"`
	Username               string                    `json:"username"`
	ImageURL               string                    `json:"image_url"`
	ProfileImageURL        string                    `json:"profile_image_url"`
	ExternalAccounts       []ClerkExternalAccount    `json:"external_accounts"`
	PrimaryEmailAddressID  string                    `json:"primary_email_address_id"`
}

type ClerkEmailAddress struct {
	EmailAddress string                 `json:"email_address"`
	ID           string                 `json:"id"`
	Verification ClerkEmailVerification `json:"verification"`
}

type ClerkEmailVerification struct {
	Status string `json:"status"`
}

type ClerkExternalAccount struct {
	Provider           string `json:"provider"`
	EmailAddress       string `json:"email_address"`
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	Username           string `json:"username"`
	Picture            string `json:"picture"`
	VerifiedAt         string `json:"verified_at"`
	GoogleID           string `json:"google_id"`
}