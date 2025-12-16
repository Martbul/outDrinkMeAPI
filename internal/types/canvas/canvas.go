package canvas

import (
	"time"
)

type CanvasItemType string

const (
	ItemTypeImage   CanvasItemType = "image"
	ItemTypeSticker CanvasItemType = "sticker"
	ItemTypeText    CanvasItemType = "text"
	ItemTypeDrawing CanvasItemType = "drawing"
)

type CanvasItem struct {
	ID              string         `json:"id"`
	DailyDrinkingID string         `json:"daily_drinking_id"`
	AddedByUserID   string         `json:"added_by_user_id"`
	ItemType        CanvasItemType `json:"item_type"`
	Content         string         `json:"content"`
	PosX            float64        `json:"pos_x"`
	PosY            float64        `json:"pos_y"`
	Rotation        float64        `json:"rotation"`
	Scale           float64        `json:"scale"`
	Width           float64        `json:"width"`
	Height          float64        `json:"height"`

	ZIndex int `json:"z_index"`

	CreatedAt time.Time `json:"created_at"`

	AuthorAvatarURL *string `json:"author_avatar_url,omitempty"`
	AuthorName      *string `json:"author_name,omitempty"`

	ExtraData map[string]interface{} `json:"extra_data,omitempty"`
}
