package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"outDrinkMeAPI/models"
)

type UserService struct {
	db *pgxpool.Pool
}

func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{db: db}
}

// InitSchema creates the users table with Clerk integration
func (s *UserService) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		clerk_id VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		username VARCHAR(30) UNIQUE NOT NULL,
		first_name VARCHAR(100) NOT NULL,
		last_name VARCHAR(100) NOT NULL,
		image_url TEXT,
		email_verified BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_clerk_id ON users(clerk_id);
	`

	_, err := s.db.Exec(ctx, query)
	return err
}

func (s *UserService) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	user := &models.User{
		ID:        uuid.New().String(),
		ClerkID:   req.ClerkID,
		Email:     req.Email,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		ImageURL:  req.ImageURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	query := `
	INSERT INTO users (id, clerk_id, email, username, first_name, last_name, image_url, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at
	`

	err := s.db.QueryRow(
		ctx,
		query,
		user.ID,
		user.ClerkID,
		user.Email,
		user.Username,
		user.FirstName,
		user.LastName,
		user.ImageURL,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(
		&user.ID,
		&user.ClerkID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ImageURL,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) GetUserByClerkID(ctx context.Context, clerkID string) (*models.User, error) {
	query := `
	SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at
	FROM users
	WHERE clerk_id = $1
	`

	user := &models.User{}
	err := s.db.QueryRow(ctx, query, clerkID).Scan(
		&user.ID,
		&user.ClerkID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ImageURL,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *UserService) UpdateProfileByClerkID(ctx context.Context, clerkID string, req *models.UpdateProfileRequest) (*models.User, error) {
	query := `
	UPDATE users
	SET 
		username = COALESCE(NULLIF($2, ''), username),
		first_name = COALESCE(NULLIF($3, ''), first_name),
		last_name = COALESCE(NULLIF($4, ''), last_name),
		image_url = COALESCE(NULLIF($5, ''), image_url),
		updated_at = NOW()
	WHERE clerk_id = $1
	RETURNING id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at
	`

	user := &models.User{}
	err := s.db.QueryRow(
		ctx,
		query,
		clerkID,
		req.Username,
		req.FirstName,
		req.LastName,
		req.ImageURL,
	).Scan(
		&user.ID,
		&user.ClerkID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ImageURL,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

func (s *UserService) DeleteUserByClerkID(ctx context.Context, clerkID string) error {
	query := `DELETE FROM users WHERE clerk_id = $1`
	
	result, err := s.db.Exec(ctx, query, clerkID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (s *UserService) UpdateEmailVerification(ctx context.Context, clerkID string, verified bool) error {
	query := `
	UPDATE users
	SET email_verified = $2, updated_at = NOW()
	WHERE clerk_id = $1
	`

	_, err := s.db.Exec(ctx, query, clerkID, verified)
	return err
}