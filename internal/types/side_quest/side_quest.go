package sidequest

import (
	"time"
)

//! Just 1 user can claim the awarn(if multyple user send the completed quest the issuer accepts them all but only the firdt one gets awarded)

// --- Enums ---

type QuestStatus string

const (
	QuestStatusOpen      QuestStatus = "OPEN"
	QuestStatusCompleted QuestStatus = "COMPLETED"
	QuestStatusExpired   QuestStatus = "EXPIRED"
	QuestStatusCancelled QuestStatus = "CANCELLED"
)

type SubmissionStatus string

const (
	SubmissionStatusPending  SubmissionStatus = "PENDING"
	SubmissionStatusApproved SubmissionStatus = "APPROVED"
	SubmissionStatusRejected SubmissionStatus = "REJECTED"
)

type PayoutStatus string

const (
	PayoutStatusPending PayoutStatus = "PENDING"
	PayoutStatusSuccess PayoutStatus = "SUCCESS"
	PayoutStatusFailed  PayoutStatus = "FAILED"
)

// --- Database Models ---

// SideQuest represents the 'side_quests' table
type SideQuest struct {
	ID              string      `db:"id"               json:"id"`
	IssuerID        string      `db:"issuer_id"        json:"issuerId"`
	IssuerName      *string     `db:"issuer_name"      json:"issuerName,omitempty"`  // Pointer for nullable
	IssuerImage     *string     `db:"issuer_image"     json:"issuerImage,omitempty"` // Pointer for nullable
	Title           string      `db:"title"            json:"title"`
	Description     string      `db:"description"      json:"description"`
	RewardAmount    float64     `db:"reward_amount"    json:"rewardAmount"`
	IsLocked        bool        `db:"is_locked"        json:"isLocked"`
	IsPublic        bool        `db:"is_public"        json:"isPublic"`
	IsAnonymous     bool        `db:"is_anonymous"     json:"isAnonymous"`
	Status          QuestStatus `db:"status"           json:"status"`
	ExpiresAt       time.Time   `db:"expires_at"       json:"expiresAt"`
	CreatedAt       time.Time   `db:"created_at"       json:"createdAt"`
	SubmissionCount int         `db:"submission_count" json:"submissionCount"`
}

// SideQuestCompletion represents the 'side_quest_completions' table
type SideQuestCompletion struct {
	ID              string           `db:"id"               json:"id"`
	SideQuestID     string           `db:"side_quest_id"    json:"sideQuestId"`
	QuestTitle      *string          `db:"quest_title"      json:"questTitle,omitempty"` // Snapshot of title
	CompleterID     string           `db:"completer_id"     json:"completerId"`
	CompleterName   *string          `db:"completer_name"   json:"completerName,omitempty"`
	CompleterImage  *string          `db:"completer_image"  json:"completerImage,omitempty"`
	ProofImageURL   string           `db:"proof_image_url"  json:"proofImageUrl"`
	ProofText       *string          `db:"proof_text"       json:"proofText,omitempty"`
	Status          SubmissionStatus `db:"status"           json:"status"`
	RejectionReason *string          `db:"rejection_reason" json:"rejectionReason,omitempty"`
	PayoutStatus    PayoutStatus     `db:"payout_status"    json:"payoutStatus"`
	PaidAt          *time.Time       `db:"paid_at"          json:"paidAt,omitempty"`
	CreatedAt       time.Time        `db:"created_at"       json:"createdAt"`
	RewardAmount    *float64         `db:"reward_amount"    json:"rewardAmount,omitempty"` // Snapshot of amount
}

// --- API Request/Response DTOs ---

// CreateSideQuestReq is the body for POST /api/v1/quests
type CreateSideQuestReq struct {
	Title         string  `json:"title" binding:"required"`
	Description   string  `json:"description" binding:"required"`
	RewardAmount  float64 `json:"rewardAmount" binding:"required,min=1"`
	DurationHours int     `json:"durationHours" binding:"required,min=1"`
	IsPublic      bool    `json:"isPublic"`
	IsAnonymous   bool    `json:"isAnonymous"`
}

// SubmitQuestProofReq is the body for POST /api/v1/quests/:id/submit
type SubmitQuestProofReq struct {
	ProofImageURL string `json:"proofImageUrl" binding:"required"`
	ProofText     string `json:"proofText"`
}

// ReviewSubmissionReq is the body for POST /api/v1/quests/submissions/:id/review
type ReviewSubmissionReq struct {
	Status          SubmissionStatus `json:"status" binding:"required,oneof=APPROVED REJECTED"`
	RejectionReason string           `json:"rejectionReason,omitempty"`
}