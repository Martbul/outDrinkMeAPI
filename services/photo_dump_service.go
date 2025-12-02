package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skip2/go-qrcode"
)

type PhotoDumpService struct {
	db *pgxpool.Pool
}

func NewPhotoDumpService(db *pgxpool.Pool) *PhotoDumpService {
	return &PhotoDumpService{
		db: db,
	}
}

type PhotoDumpSessionResponse struct {
	SessionID    uuid.UUID `json:"session_id"`
	QrToken      string    `json:"qr_token"`
	QrCodeBase64 string    `json:"qr_code_base64"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (s *PhotoDumpService) GenerateQrCode(ctx context.Context, clerkID string) (*PhotoDumpSessionResponse, error) {
	// 1. Get the internal User UUID from the Clerk ID
	var hostUserID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&hostUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with clerk_id: %s", clerkID)
		}
		return nil, fmt.Errorf("database error fetching user: %w", err)
	}

	qrToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	query := `
		INSERT INTO photo_dump (host_user_id, qr_token, status, expires_at)
		VALUES ($1, $2, 'ACTIVE', $3)
		RETURNING id
	`
	var sessionID uuid.UUID
	err = s.db.QueryRow(ctx, query, hostUserID, qrToken, expiresAt).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create photo dump session: %w", err)
	}

	qrContent := fmt.Sprintf("outdrinkme://photodump/session/join/%s", qrToken)

	pngBytes, err := qrcode.Encode(qrContent, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR png: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(pngBytes)

	return &PhotoDumpSessionResponse{
		SessionID:    sessionID,
		QrToken:      qrToken,
		QrCodeBase64: base64Image,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *PhotoDumpService) JoinViaQrCode(ctx context.Context, clerkID string) (bool, error) {
	return false, nil

}

func (s *PhotoDumpService) GetSessionData(ctx context.Context, clerkID string) (int,error) {
	return 0,nil
}

func (s *PhotoDumpService) AddImages(ctx context.Context, clerkID string) (int, error) {
	return 0, nil
}
