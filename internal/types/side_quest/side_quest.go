package sidequest

import (
	"time"
)

//! Just 1 user can claim the awarn(if multyple user send the completed quest the issuer accepts them all but only the firdt one gets awarded)


type QuestStatus string
type SubmissionStatus string
type PayoutStatus string

const (
	QuestStatusOpen      QuestStatus = "OPEN"       // Available for anyone
	QuestStatusCompleted QuestStatus = "COMPLETED"  // Finished and paid out
	QuestStatusExpired   QuestStatus = "EXPIRED"    // Time run out, money returned to issuer
	QuestStatusCancelled QuestStatus = "CANCELLED"  // Issuer changed mind before anyone tried

	SubmissionPending  SubmissionStatus = "PENDING"  // Waiting for Issuer to review
	SubmissionApproved SubmissionStatus = "APPROVED" // Issuer said "Yes"
	SubmissionRejected SubmissionStatus = "REJECTED" // Issuer said "No/Fake proof"

	PayoutPending    PayoutStatus = "PENDING"
	PayoutSuccess    PayoutStatus = "SUCCESS"
	PayoutFailed     PayoutStatus = "FAILED"     // System error during gem transfer
)


type SideQuest struct {
	ID          string      `json:"id" db:"id"`
	IssuerID    string      `json:"issuerId" db:"issuer_id"`     // Who is paying
	Title       string      `json:"title" db:"title"`            // e.g., "Find me a charger"
	Description string      `json:"description" db:"description"`
	
	RewardAmount int  `json:"rewardAmount" db:"reward_amount"` // Amount of Gems/Shame
	IsLocked     bool `json:"isLocked" db:"is_locked"`         // Has the issuer's currency been reserved?
	
	IsPublic      bool    `json:"isPublic" db:"is_public"`           // false = Friend Board, true = Public Zoo
	IsAnonymous   bool    `json:"isAnonymous" db:"is_anonymous"`     // If true, IssuerID is hidden in UI
	
	Status    QuestStatus `json:"status" db:"status"`
	ExpiresAt time.Time   `json:"expiresAt" db:"expires_at"` 
	CreatedAt time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time   `json:"updatedAt" db:"updated_at"`
}

type SideQuestCompletion struct {
	ID          string `json:"id" db:"id"`
	SideQuestID string `json:"sideQuestId" db:"side_quest_id"`
	CompleterID string `json:"completerId" db:"completer_id"` // Who did the quest
	
	// Proof (The "Receipt")
	ProofImageURL string `json:"proofImageUrl" db:"proof_image_url"`
	ProofText     string `json:"proofText,omitempty" db:"proof_text"` // Optional comment
	
	// Approval Workflow
	Status         SubmissionStatus `json:"status" db:"status"`                 // Pending/Approved/Rejected
	RejectionReason string          `json:"rejectionReason,omitempty" db:"rejection_reason"`
	
	// Financial Settlement
	PayoutStatus PayoutStatus `json:"payoutStatus" db:"payout_status"`
	PaidAt       *time.Time   `json:"paidAt,omitempty" db:"paid_at"`
	
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

