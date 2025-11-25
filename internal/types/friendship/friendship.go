package friendship

import (
	"time"

	"github.com/google/uuid"
)

type FriendshipStatus string

const (
	FriendshipPending  FriendshipStatus = "pending"
	FriendshipAccepted FriendshipStatus = "accepted"
	FriendshipBlocked  FriendshipStatus = "blocked"
)

type Friendship struct {
	ID        uuid.UUID        `json:"id" db:"id"`
	UserID    uuid.UUID        `json:"user_id" db:"user_id"`
	FriendID  uuid.UUID        `json:"friend_id" db:"friend_id"`
	Status    FriendshipStatus `json:"status" db:"status"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`
}