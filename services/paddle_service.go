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

func NewPaddleService(client *paddle.SDK, db *pgxpool.Pool) *PaddleService {
	return &PaddleService{
		PaddleClient: client,
		db:           db,
	}
}

func (s *PaddleService) UnlockPremium(ctx context.Context, userID string, validUntil time.Time, transactionID, customerID, amount, currency string) error {
	var username, imageURL string
	err := s.db.QueryRow(ctx, "SELECT username, image_url FROM users WHERE clerk_id = $1", userID).Scan(&username, &imageURL)
	if err != nil {
		return fmt.Errorf("failed to find user %s: %w", userID, err)
	}

	query := `
		INSERT INTO premium (
			user_id, username, user_image_url, valid_until, is_active, 
			transaction_id, customer_id, amount, currency, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7, $8, $9, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			valid_until    = EXCLUDED.valid_until,
			is_active      = true,
			transaction_id = EXCLUDED.transaction_id,
			customer_id    = EXCLUDED.customer_id,
			amount         = EXCLUDED.amount,
			currency       = EXCLUDED.currency,
			updated_at     = NOW();
	`

	_, err = s.db.Exec(ctx, query, userID, username, imageURL, validUntil, transactionID, customerID, amount, currency)
	if err != nil {
		return fmt.Errorf("failed to unlock premium: %w", err)
	}

	return nil
}

func (s *PaddleService) RevokePremium(ctx context.Context, subscriptionID string) error {
	query := `UPDATE premium SET is_active = false, updated_at = NOW() WHERE subscription_id = $1`
	
	_, err := s.db.Exec(ctx, query, subscriptionID)
	return err
}