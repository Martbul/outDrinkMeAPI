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

type FuncService struct {
	db *pgxpool.Pool
}

func NewFuncService(db *pgxpool.Pool) *FuncService {
	return &FuncService{
		db: db,
	}
}

// FuncImage represents a single image record within a function session
type FuncImage struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"image_url"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// FuncDataResponse is returned when fetching the full session details
type FuncDataResponse struct {
	InviteCode string      `json:"inviteCode"`
	ExpiresAt  time.Time   `json:"expiresAt"`
	Images     []FuncImage `json:"images"`
}

// FuncServiceSessionResponse is returned specifically after a successful creation
type FuncServiceSessionResponse struct {
	SessionID    uuid.UUID `json:"sessionID"`
	QrToken      string    `json:"qrToken"`
	QrCodeBase64 string    `json:"qrCodeBase64"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (s *FuncService) GenerateQrCode(ctx context.Context, clerkID string) (*FuncServiceSessionResponse, error) {
	var hostUserID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&hostUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found with clerk_id: %s", clerkID)
		}
		return nil, fmt.Errorf("database error fetching user: %w", err)
	}

	qrToken := uuid.New().String()
	expiresAt := time.Now().Add(72 * time.Hour) 

	query := `
		INSERT INTO funcs (host_user_id, qr_token, status, expires_at)
		VALUES ($1, $2, 'ACTIVE', $3)
		RETURNING id
	`
	var sessionID uuid.UUID
	err = s.db.QueryRow(ctx, query, hostUserID, qrToken, expiresAt).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create func session: %w", err)
	}

	qrContent := fmt.Sprintf("outdrinkme://photodump/session/join/%s", qrToken)

	pngBytes, err := qrcode.Encode(qrContent, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR png: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(pngBytes)

	return &FuncServiceSessionResponse{
		SessionID:    sessionID,
		QrToken:      qrToken,
		QrCodeBase64: base64Image,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *FuncService) JoinViaQrCode(ctx context.Context, clerkID string, qrToken string) (uuid.UUID, error) {
	var funcID uuid.UUID
	var userID uuid.UUID

	// 1. Get User ID and Func ID
	err := s.db.QueryRow(ctx, `
		SELECT f.id, u.id FROM funcs f, users u 
		WHERE f.qr_token = $1 AND u.clerk_id = $2 AND f.expires_at > NOW()`,
		qrToken, clerkID).Scan(&funcID, &userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("session not found or expired")
	}

	// 2. Add to members (Ignore if already member)
	_, err = s.db.Exec(ctx, `
		INSERT INTO func_members (func_id, user_id) 
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, funcID, userID)

	return funcID, err
}

func (s *FuncService) GetSessionData(ctx context.Context, funcID string) (*FuncDataResponse, error) {
	resp := &FuncDataResponse{
		Images: []FuncImage{},
	}

	err := s.db.QueryRow(ctx, `
		SELECT qr_token, expires_at 
		FROM funcs 
		WHERE id = $1`, funcID).Scan(&resp.InviteCode, &resp.ExpiresAt)

	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT i.id, i.image_url, u.username, i.created_at 
		FROM funcs_images i 
		JOIN users u ON i.user_id = u.id 
		WHERE i.func_id = $1 
		ORDER BY i.created_at DESC`, funcID)

	if err != nil {
		return resp, nil // Return meta even if images fail
	}
	defer rows.Close()

	for rows.Next() {
		var img FuncImage
		if err := rows.Scan(&img.ID, &img.URL, &img.Username, &img.CreatedAt); err == nil {
			resp.Images = append(resp.Images, img)
		}
	}

	return resp, nil
}

func (s *FuncService) AddImages(ctx context.Context, clerkID string, funcID string, imageURL string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO funcs_images (func_id, user_id, image_url)
		VALUES ($1, (SELECT id FROM users WHERE clerk_id = $2), $3)`,
		funcID, clerkID, imageURL)
	return err
}
