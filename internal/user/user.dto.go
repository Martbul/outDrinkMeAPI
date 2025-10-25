package user

import (
	"outDrinkMeAPI/internal/achievement"
	"outDrinkMeAPI/internal/stats"
)

type CreateUserRequest struct {
	ClerkID   string `json:"clerkId" validate:"required"`
	Email     string `json:"email" validate:"required,email"`
	Username  string `json:"username" validate:"required,min=3,max=30"`
	FirstName string `json:"firstName" validate:"required"`
	LastName  string `json:"lastName" validate:"required"`
	ImageURL  string `json:"imageUrl,omitempty"`
}

type UpdateProfileRequest struct {
	Username  string `json:"username,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	ImageURL  string `json:"imageUrl,omitempty"`
	Gems      int    `json:"gems,omitempty"`
}

type AddFriend struct {
	FriendId string `json:"friendId,omitempty"`
}



type FriendDeiscoveryProfileRequest struct {
	FriendDiscoveryId string `json:"friendDiscoveryId,omitempty"`
}

type FriendDiscoveryDisplayProfileResponse struct {
	User         *User                           `json:"user"`
	Stats        *stats.UserStats                     `json:"stats"`
	Achievements []*achievement.AchievementWithStatus `json:"achievements"`
	IsFriend     bool                                 `json:"is_friend"`
}
