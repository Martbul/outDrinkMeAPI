package models

import "time"

type User struct {
	ID             string    `json:"id"`
	ClerkID        string    `json:"clerkId"`
	Email          string    `json:"email"`
	Username       string    `json:"username"`
	FirstName      string    `json:"firstName"`
	LastName       string    `json:"lastName"`
	ImageURL       string    `json:"imageUrl,omitempty"`
	EmailVerified  bool      `json:"emailVerified"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

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
}