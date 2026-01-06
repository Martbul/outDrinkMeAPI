package services

import (
	"context"
	"fmt"
	"time"

	paddle "github.com/PaddleHQ/paddle-go-sdk"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaddleService struct {
	PaddleClient *paddle.SDK
	db           *pgxpool.Pool
}

// NewPaddleService is a constructor helper (optional, but good practice)
func NewPaddleService(client *paddle.SDK, db *pgxpool.Pool) *PaddleService {
	return &PaddleService{
		PaddleClient: client,
		db:           db,
	}
}

func (s *PaddleService) UnlockPremium(ctx context.Context, userID string, validUntil time.Time) error {
	// 1. Fetch User Details
	// pgx: QueryRow takes context as the first argument
	var username, imageURL string
	err := s.db.QueryRow(ctx, "SELECT username, image_url FROM users WHERE clerk_id = $1", userID).Scan(&username, &imageURL)
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", userID, err)
	}

	// 2. Generate QR Code Data
	qrData := fmt.Sprintf("ODM-PREM-%s-%d", userID, time.Now().Unix())

	// 3. Upsert Premium Record
	// Note: We set subscription_id to NULL because this is a one-time purchase
	query := `
		INSERT INTO premium (user_id, username, user_image_url, qr_code_data, valid_until, is_active, subscription_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, NULL, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			valid_until = EXCLUDED.valid_until,
			is_active = true,
			subscription_id = NULL, -- Clear old subs if they buy a one-time pass
			updated_at = NOW();
	`

	// pgx: Exec takes context as the first argument
	_, err = s.db.Exec(ctx, query, userID, username, imageURL, qrData, validUntil)
	if err != nil {
		return fmt.Errorf("failed to unlock premium: %w", err)
	}

	return nil
}

func (s *PaddleService) RevokePremium(ctx context.Context, subscriptionID string) error {
	query := `UPDATE premium SET is_active = false, updated_at = NOW() WHERE subscription_id = $1`
	
	// pgx: Exec takes context as the first argument
	_, err := s.db.Exec(ctx, query, subscriptionID)
	return err
}