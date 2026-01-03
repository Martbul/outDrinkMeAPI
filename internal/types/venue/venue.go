 package venue

import (
	"time"
)

type VenueCategory string

const (
	CategoryClub       VenueCategory = "Club"
	CategoryBar        VenueCategory = "Bar"
	CategoryChalgaClub VenueCategory = "Chalga Club"
	CategoryPianoBar   VenueCategory = "Piano Bar"
	CategoryBeachBar   VenueCategory = "Beach Bar"
	CategoryRooftop    VenueCategory = "Rooftop"
	CategoryPub        VenueCategory = "Pub"
	CategoryLounge     VenueCategory = "Lounge"
)

type VenueSpecial struct {
	ID          string `db:"id"          json:"id"`
	VenueID     string `db:"venue_id"    json:"venueId"`
	Name        string `db:"name"        json:"name"`
	Price       string `db:"price"       json:"price"`
	Description string `db:"description" json:"description"`
	ImageURL    string `db:"image_url"   json:"imageUrl"`
}

type Venue struct {
	ID                 string        `db:"id"                  json:"id"`
	Name               string        `db:"name"                json:"name"`
	VenueType          VenueCategory `db:"venue_type"          json:"venueType"`
	ImageURL           string        `db:"image_url"           json:"imageUrl"`
	ImageWidth         int           `db:"image_width"         json:"imageWidth"`
	ImageHeight        int           `db:"image_height"        json:"imageHeight"`
	Location           string        `db:"location"            json:"location"`
	DistanceKm         float64       `db:"distance_km"         json:"distance"` // Mapped to JSON 'distance'
	DistanceStr        string        `db:"distance_str"        json:"distanceStr"`
	Rating             float64       `db:"rating"              json:"rating"`
	ReviewCount        int           `db:"review_count"        json:"reviewCount"`
	Difficulty         string        `db:"difficulty"          json:"difficulty"`
	EventTime          string        `db:"event_time"          json:"time"` // Mapped to JSON 'time'
	Description        string        `db:"description"         json:"description"`
	Latitude           float64       `db:"latitude"            json:"latitude"`
	Longitude          float64       `db:"longitude"           json:"longitude"`
	
	Tags               []string      `db:"tags"                json:"tags"` 
	
	DiscountPercentage int           `db:"discount_percentage" json:"discount"`

	CreatedAt          time.Time     `db:"created_at"          json:"createdAt"`

	Specials           []VenueSpecial `db:"-"                  json:"specials,omitempty"`
	Employees          []string       `db:"-"                  json:"employees,omitempty"`
}

type VenueResponse struct {
	Venue // Embed the main struct
	Coordinates struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinates"`
}