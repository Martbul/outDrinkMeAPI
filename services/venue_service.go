package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"outDrinkMeAPI/internal/types/venue"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VenueService struct {
	db *pgxpool.Pool
}

func NewVenueService(db *pgxpool.Pool) *VenueService {
	return &VenueService{db: db}
}
func (s *VenueService) GetAllVenues(ctx context.Context) ([]venue.Venue, error) {
	query := `
		SELECT
			v.id,
			v.name,
			v.venue_type,
			v.image_url,
			v.image_width,
			v.image_height,
			v.location,
			v.distance_km,
			v.distance_str,
			v.rating,
			v.review_count,
			v.difficulty,
			v.event_time,
			v.description,
			v.latitude,
			v.longitude,
			v.tags,
			v.discount_percentage,
			
			-- New Columns with safety Coalesce
			COALESCE(v.gallery, '{}') AS gallery,
			COALESCE(v.features, '{}') AS features,
			COALESCE(v.phone, '') AS phone,
			COALESCE(v.website, '') AS website,
			COALESCE(v.directions, '') AS directions,

			-- Subquery to get Specials as a JSON Array
			COALESCE((
				SELECT json_agg(json_build_object(
					'id', vs.id,
					'venue_id', vs.venue_id,
					'name', vs.name,
					'price', vs.price,
					'description', vs.description,
					'image_url', vs.image_url
				))
				FROM venue_specials vs
				WHERE vs.venue_id = v.id
			), '[]'::json) AS specials,

			-- Subquery to get Employee IDs as a Text Array
			COALESCE((
				SELECT array_agg(ve.user_id)
				FROM venue_employees ve
				WHERE ve.venue_id = v.id
			), '{}'::text[]) AS employees

		FROM venues v
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query venues: %w", err)
	}
	defer rows.Close()

	var venues []venue.Venue

	for rows.Next() {
		var v venue.Venue
		var specialsJSON []byte 

		err := rows.Scan(
			&v.ID,
			&v.Name,
			&v.VenueType,
			&v.ImageURL,
			&v.ImageWidth,
			&v.ImageHeight,
			&v.Location,
			&v.DistanceKm,
			&v.DistanceStr,
			&v.Rating,
			&v.ReviewCount,
			&v.Difficulty,
			&v.EventTime,
			&v.Description,
			&v.Latitude,
			&v.Longitude,
			&v.Tags,               
			&v.DiscountPercentage,
			// New Fields
			&v.Gallery,
			&v.Features,
			&v.Phone,
			&v.Website,
			&v.Directions,
			// Subqueries
			&specialsJSON,        
			&v.Employees,        
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan venue row: %w", err)
		}

		if len(specialsJSON) > 0 {
			if err := json.Unmarshal(specialsJSON, &v.Specials); err != nil {
				return nil, fmt.Errorf("failed to unmarshal specials: %w", err)
			}
		}

		venues = append(venues, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return venues, nil
}
type EmployeeDetails struct {
	Role  string `json:"role"`
	Scans int    `json:"scans"`
}

func (s *VenueService) GetEmployeeDetails(ctx context.Context, clerkID string) (*EmployeeDetails, error) {
	query := `
		SELECT role, COALESCE(scans, 0)
		FROM venue_employees
		WHERE user_id = $1
		LIMIT 1
	`

	var details EmployeeDetails
	
	err := s.db.QueryRow(ctx, query, clerkID).Scan(&details.Role, &details.Scans)
	if err != nil {
		return nil, fmt.Errorf("failed to get employee details: %w", err)
	}

	return &details, nil
}

func (s *VenueService) AddEmployeeToVenue(ctx context.Context, venueID string, userID string, role string) (bool, error) {
	query := `
		INSERT INTO venue_employees (venue_id, user_id, role, scans)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (venue_id, user_id) DO NOTHING
	`

	tag, err := s.db.Exec(ctx, query, venueID, userID, role)
	if err != nil {
		return false, fmt.Errorf("failed to add employee: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (s *VenueService) RemoveEmployeeFromVenue(ctx context.Context, venueID string, userID string) (bool, error) {
	query := `
		DELETE FROM venue_employees
		WHERE venue_id = $1 AND user_id = $2
	`

	tag, err := s.db.Exec(ctx, query, venueID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to remove employee: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}
type ScanDataReq struct {
	VenueID            string
	ScannerUserID      string
	Token              string // The QR Code String
	DiscountPercentage string
}

type ScanSuccessResponse struct {
	CustomerUsername string `json:"username"`
	CustomerImage    string `json:"image"`
	Message          string `json:"message"`
}

// QR Payload structure (Must match what generates the QR)
type QRTokenPayload struct {
	UserID    string `json:"uid"`
	ExpiresAt int64  `json:"exp"`
}

func (s *VenueService) ProcessScan(ctx context.Context, req ScanDataReq) (*ScanSuccessResponse, error) {
	parts := strings.Split(req.Token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	payloadStr, signature := parts[0], parts[1]

	// B. Verify Signature
	secretKey := os.Getenv("QR_SIGNING_SECRET")
	hMac := hmac.New(sha256.New, []byte(secretKey))
	hMac.Write([]byte(payloadStr))
	expectedSig := base64.RawURLEncoding.EncodeToString(hMac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// C. Decode Payload
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(payloadStr)
	var payload QRTokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("malformed token payload")
	}

	// D. Check Expiration (Prevents Screenshots)
	if time.Now().Unix() > payload.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	customerID := payload.UserID

	// --- STEP 2: CHECK PREMIUM STATUS IN DB ---

	var isActive bool
	var validUntil time.Time
	var username, userImage string

	// We check premium table AND join users to get the name/image for the UI
	// pgxpool.QueryRow works just like database/sql
	checkQuery := `
		SELECT p.is_active, p.valid_until, p.username, p.user_image_url
		FROM premium p
		WHERE p.user_id = $1
	`
	err := s.db.QueryRow(ctx, checkQuery, customerID).Scan(&isActive, &validUntil, &username, &userImage)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user has no premium record")
		}
		return nil, fmt.Errorf("db error: %w", err)
	}

	// Logic Check
	if !isActive {
		return nil, fmt.Errorf("premium not active")
	}
	if time.Now().After(validUntil) {
		return nil, fmt.Errorf("premium expired on %s", validUntil.Format("2006-01-02"))
	}

	// --- STEP 3: PERFORM SCAN TRANSACTION ---

	// pgxpool.Begin returns a pgx.Tx
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// A. Insert into History
	insertQuery := `
		INSERT INTO venue_scans (venue_id, customer_user_id, scanner_user_id, discount_percentage, scanned_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err = tx.Exec(ctx, insertQuery, req.VenueID, customerID, req.ScannerUserID, req.DiscountPercentage)
	if err != nil {
		return nil, fmt.Errorf("failed to insert scan record: %w", err)
	}

	// B. Increment Employee Scan Count
	updateEmployeeQuery := `
		UPDATE venue_employees 
		SET scans = COALESCE(scans, 0) + 1 
		WHERE venue_id = $1 AND user_id = $2
	`
	tag, err := tx.Exec(ctx, updateEmployeeQuery, req.VenueID, req.ScannerUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to update employee stats: %w", err)
	}
	// Strict check: Is the scanner actually an employee?
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("scanner is not authorized at this venue")
	}

	// C. Update User's Premium Stats (Venues Visited)
	updateUserQuery := `
		UPDATE premium
		SET venues_visited = venues_visited + 1
		WHERE user_id = $1
	`
	_, err = tx.Exec(ctx, updateUserQuery, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user stats: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("transaction commit failed: %w", err)
	}

	return &ScanSuccessResponse{
		CustomerUsername: username,
		CustomerImage:    userImage,
		Message:          "Scan Valid",
	}, nil
}