package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// --- Struct Definitions for JSON Responses ---

type FuncMember struct {
	Username string `json:"username"`
	ImageUrl string `json:"imageUrl"`
}

type FuncMetadata struct {
	InviteCode   string    `json:"inviteCode"`
	QrToken      string    `json:"qrToken"`     
	QrCodeBase64 string    `json:"qrCodeBase64"`
	ExpiresAt    time.Time `json:"expiresAt"`
	SessionID    string    `json:"sessionID"`
	HostUsername string    `json:"hostUsername"`
	HostImageUrl string    `json:"hostImageUrl"`
}

type FuncDataResponse struct {
	IsPartOfActiveFunc bool         `json:"isPartOfActiveFunc"`
	FuncMembers        []FuncMember `json:"funcMembers"`
	FuncImageIds       []string     `json:"funcImageIds"`
	FuncMetadata       FuncMetadata `json:"funcMetadata"`
}

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
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found with clerk_id: %s", clerkID)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	qrToken := uuid.New().String()
	expiresAt := time.Now().Add(72 * time.Hour) // 3-day lifespan

	// Start a transaction using pgxpool
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Create the Function entry
	var sessionID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO funcs (host_user_id, qr_token, status, expires_at)
		VALUES ($1, $2, 'ACTIVE', $3)
		RETURNING id
	`, hostUserID, qrToken, expiresAt).Scan(&sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert func: %w", err)
	}

	// Join the Host to the member list immediately
	_, err = tx.Exec(ctx, `
		INSERT INTO func_members (func_id, user_id)
		VALUES ($1, $2)
	`, sessionID, hostUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to add host to members: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Generate QR Code
	qrContent := fmt.Sprintf("outdrinkme://photodump/session/join/%s", qrToken)
	pngBytes, err := qrcode.Encode(qrContent, qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR png: %w", err)
	}

	return &FuncServiceSessionResponse{
		SessionID:    sessionID,
		QrToken:      qrToken,
		QrCodeBase64: base64.StdEncoding.EncodeToString(pngBytes),
		ExpiresAt:    expiresAt,
	}, nil
}

// 2. JoinViaQrCode adds the current user to the member list of an active session.
func (s *FuncService) JoinViaQrCode(ctx context.Context, clerkID string, qrToken string) (uuid.UUID, error) {
	var funcID uuid.UUID
	var userID uuid.UUID

	// Check if session is valid and get user id
	err := s.db.QueryRow(ctx, `
		SELECT f.id, u.id 
		FROM funcs f, users u 
		WHERE f.qr_token = $1 
		AND u.clerk_id = $2 
		AND f.expires_at > NOW()`,
		qrToken, clerkID).Scan(&funcID, &userID)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, fmt.Errorf("this party has expired or doesn't exist")
		}
		return uuid.Nil, err
	}

	// Insert into members
	_, err = s.db.Exec(ctx, `
		INSERT INTO func_members (func_id, user_id) 
		VALUES ($1, $2) 
		ON CONFLICT (func_id, user_id) DO NOTHING`, 
		funcID, userID)
	
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to join group: %w", err)
	}

	return funcID, nil
}

func (s *FuncService) GetSessionData(ctx context.Context, funcID string, currentClerkID string) (*FuncDataResponse, error) {
	resp := &FuncDataResponse{
		FuncMembers:  []FuncMember{},
		FuncImageIds: []string{},
	}

	// 1. Get Metadata and Host Info
	err := s.db.QueryRow(ctx, `
		SELECT 
			f.qr_token, f.expires_at, f.id, 
			u.username, COALESCE(u.image_url, '')
		FROM funcs f
		JOIN users u ON f.host_user_id = u.id
		WHERE f.id = $1`, funcID).Scan(
		&resp.FuncMetadata.InviteCode,
		&resp.FuncMetadata.ExpiresAt,
		&resp.FuncMetadata.SessionID,
		&resp.FuncMetadata.HostUsername,
		&resp.FuncMetadata.HostImageUrl,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// --- NEW: REGENERATE QR DATA FOR METADATA ---
	resp.FuncMetadata.QrToken = resp.FuncMetadata.InviteCode
	qrContent := fmt.Sprintf("outdrinkme://photodump/session/join/%s", resp.FuncMetadata.QrToken)
	pngBytes, err := qrcode.Encode(qrContent, qrcode.Medium, 256)
	if err == nil {
		resp.FuncMetadata.QrCodeBase64 = base64.StdEncoding.EncodeToString(pngBytes)
	}
	// --------------------------------------------

	// 2. Check if the requesting user is a member
	err = s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM func_members fm 
			JOIN users u ON fm.user_id = u.id 
			WHERE fm.func_id = $1 AND u.clerk_id = $2
		)`, funcID, currentClerkID).Scan(&resp.IsPartOfActiveFunc)
	if err != nil {
		return nil, err
	}

	// 3. Get All Members
	rows, err := s.db.Query(ctx, `
		SELECT u.username, COALESCE(u.image_url, '')
		FROM func_members fm
		JOIN users u ON fm.user_id = u.id
		WHERE fm.func_id = $1
		ORDER BY fm.joined_at ASC`, funcID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m FuncMember
			if err := rows.Scan(&m.Username, &m.ImageUrl); err == nil {
				resp.FuncMembers = append(resp.FuncMembers, m)
			}
		}
	}

	// 4. Get Image URLs
	imgRows, err := s.db.Query(ctx, `
		SELECT image_url FROM funcs_images 
		WHERE func_id = $1 
		ORDER BY created_at DESC`, funcID)
	if err == nil {
		defer imgRows.Close()
		for imgRows.Next() {
			var url string
			if err := imgRows.Scan(&url); err == nil {
				resp.FuncImageIds = append(resp.FuncImageIds, url)
			}
		}
	}

	return resp, nil
}

func (s *FuncService) AddImages(ctx context.Context, clerkID string, funcID string, imageUrls []string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, url := range imageUrls {
		_, err := tx.Exec(ctx, `
			INSERT INTO funcs_images (func_id, user_id, image_url)
			VALUES ($1, $2, $3)`,
			funcID, userID, url)
		if err != nil {
			return fmt.Errorf("failed to insert image %s: %w", url, err)
		}
	}

	return tx.Commit(ctx)
}

func (s *FuncService) DeleteImages(ctx context.Context, clerkID string, funcID string, imageUrls []string) error {
	// 1. Get User ID from Clerk ID
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// 2. Start Transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 3. Delete Images
	// We verify ownership by checking user_id in the WHERE clause.
	// If funcID is provided, we also scope it to that function.
	query := `DELETE FROM funcs_images WHERE image_url = $1 AND user_id = $2`
	
	// If the frontend provided a FuncID, we add it to the constraint for extra safety
	if funcID != "" {
		query += ` AND func_id = $3`
	}

	for _, url := range imageUrls {
		var err error
		// var result interface{} // Placeholder for Exec result if needed

		if funcID != "" {
			_, err = tx.Exec(ctx, query, url, userID, funcID)
		} else {
			_, err = tx.Exec(ctx, query, url, userID)
		}

		if err != nil {
			return fmt.Errorf("failed to delete image %s: %w", url, err)
		}
	}

	// 4. Commit Transaction
	return tx.Commit(ctx)
}

func (s *FuncService) GetUserActiveSession(ctx context.Context, clerkID string) (*FuncDataResponse, error) {
	var funcID uuid.UUID
    
    // Find the most recent active session this user is a member of
	err := s.db.QueryRow(ctx, `
		SELECT fm.func_id 
		FROM func_members fm
		JOIN funcs f ON fm.func_id = f.id
		JOIN users u ON fm.user_id = u.id
		WHERE u.clerk_id = $1 AND f.expires_at > NOW()
		ORDER BY f.created_at DESC LIMIT 1`, clerkID).Scan(&funcID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return &FuncDataResponse{IsPartOfActiveFunc: false}, nil
		}
		return nil, err
	}

	return s.GetSessionData(ctx, funcID.String(), clerkID)
}


func (s *FuncService) LeaveFunction(ctx context.Context, clerkID string, funcID string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM func_members 
		WHERE func_id = $1 
		AND user_id = (SELECT id FROM users WHERE clerk_id = $2)`,
		funcID, clerkID)
	
	if err != nil {
		return fmt.Errorf("failed to leave function: %w", err)
	}
	return nil
}