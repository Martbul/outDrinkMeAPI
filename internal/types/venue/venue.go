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
	VenueID     string `db:"venue_id"    json:"venue_id"`     
	Name        string `db:"name"        json:"name"`
	Price       string `db:"price"       json:"price"`
	Description string `db:"description" json:"description"`
	ImageURL    string `db:"image_url"   json:"image_url"`    
}

type Venue struct {
	ID                 string        `db:"id"                  json:"id"`
	Name               string        `db:"name"                json:"name"`
	VenueType          VenueCategory `db:"venue_type"          json:"venue_type"`       
	ImageURL           string        `db:"image_url"           json:"image_url"`        
	ImageWidth         int           `db:"image_width"         json:"image_width"`      
	ImageHeight        int           `db:"image_height"        json:"image_height"`     
	Location           string        `db:"location"            json:"location"`
	
	DistanceKm         float64       `db:"distance_km"         json:"distance_km"`      
	DistanceStr        string        `db:"distance_str"        json:"distance_str"`     
	
	Rating             float64       `db:"rating"              json:"rating"`
	ReviewCount        int           `db:"review_count"        json:"review_count"`     
	Difficulty         string        `db:"difficulty"          json:"difficulty"`
	
	EventTime          string        `db:"event_time"          json:"event_time"`       
	Description        string        `db:"description"         json:"description"`
	Latitude           float64       `db:"latitude"            json:"latitude"`
	Longitude          float64       `db:"longitude"           json:"longitude"`
	
	Tags               []string      `db:"tags"                json:"tags"` 
	
	DiscountPercentage int           `db:"discount_percentage" json:"discount_percentage"` 

	CreatedAt          time.Time     `db:"created_at"          json:"created_at"`       

	Specials           []VenueSpecial `db:"-"                  json:"specials,omitempty"`
	Employees          []string       `db:"-"                  json:"employees,omitempty"`
}

type VenueResponse struct {
	Venue 
	Coordinates struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinates"`
}