package services

import (
	"context"
	"encoding/json"
	"fmt"
	"outDrinkMeAPI/internal/types/venue"

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


// ScanDataReq holds the data needed to record a scan
type ScanDataReq struct {
	VenueID           string
	CustomerID        string
	ScannerUserID     string
	DiscountPercentage string
}

//TODO! Check if user has active premium
func (s *VenueService) AddScanData(ctx context.Context, req ScanDataReq) (bool, error) {
	// Start a transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Defer rollback in case of error (if Commit is called, Rollback is a no-op)
	defer tx.Rollback(ctx)

	// 1. Insert into venue_scans history table
	insertQuery := `
		INSERT INTO venue_scans (venue_id, customer_user_id, scanner_user_id, discount_percentage)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, insertQuery, req.VenueID, req.CustomerID, req.ScannerUserID, req.DiscountPercentage)
	if err != nil {
		return false, fmt.Errorf("failed to insert scan record: %w", err)
	}

	// 2. Increment the scan count for the employee in venue_employees
	// We use COALESCE to handle cases where 'scans' might be NULL initially
	updateQuery := `
		UPDATE venue_employees 
		SET scans = COALESCE(scans, 0) + 1 
		WHERE venue_id = $1 AND user_id = $2
	`
	tag, err := tx.Exec(ctx, updateQuery, req.VenueID, req.ScannerUserID)
	if err != nil {
		return false, fmt.Errorf("failed to update employee scan count: %w", err)
	}

	// Optional: Check if the employee actually existed in that venue
	if tag.RowsAffected() == 0 {
		return false, fmt.Errorf("scanner user is not an employee of this venue")
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return true, nil
}