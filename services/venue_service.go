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
	// We use subqueries to aggregate Specials (as JSON) and Employees (as Array)
	// This avoids the N+1 query problem.
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
					'venueId', vs.venue_id,
					'name', vs.name,
					'price', vs.price,
					'description', vs.description,
					'imageUrl', vs.image_url
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
		var specialsJSON []byte // Temp holder for the raw JSON

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
			&v.Tags,               // pgx handles text[] -> []string automatically
			&v.DiscountPercentage,
			&specialsJSON,         // Scan the JSON blob
			&v.Employees,          // pgx handles text[] -> []string automatically
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan venue row: %w", err)
		}

		// Unmarshal the Specials JSON into the struct slice
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