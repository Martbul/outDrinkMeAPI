package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"outDrinkMeAPI/internal/types/achievement"
	"outDrinkMeAPI/internal/types/calendar"
	"outDrinkMeAPI/internal/types/canvas"
	"outDrinkMeAPI/internal/types/collection"
	"outDrinkMeAPI/internal/types/leaderboard"
	"outDrinkMeAPI/internal/types/mix"
	"outDrinkMeAPI/internal/types/stats"
	"outDrinkMeAPI/internal/types/store"
	"outDrinkMeAPI/internal/types/story"
	"outDrinkMeAPI/internal/types/subscription"
	"outDrinkMeAPI/internal/types/user"
	"outDrinkMeAPI/utils"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	stripeClient "github.com/stripe/stripe-go/v76/subscription"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	db           *pgxpool.Pool
	notifService *NotificationService
}

func NewUserService(db *pgxpool.Pool, notifService *NotificationService) *UserService {
	return &UserService{
		db:           db,
		notifService: notifService,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.User, error) {
	user := &user.User{
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

func (s *UserService) GetUserByClerkID(ctx context.Context, clerkID string) (*user.User, error) {
	query := `
	SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at, gems, xp, all_days_drinking_count, alcoholism_coefficient
	FROM users
	WHERE clerk_id = $1
	`

	user := &user.User{}
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
		&user.Gems,
		&user.XP,
		&user.AllDaysDrinkingCount,
		&user.AlcoholismCoefficient,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}
func (s *UserService) FriendDiscoveryDisplayProfile(ctx context.Context, clerkID string, FriendDiscoveryId string) (*mix.FriendDiscoveryDisplayProfileResponse, error) {
	var currnetUserID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&currnetUserID)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Failed to find requesting user: %v", err)
		return nil, fmt.Errorf("user not authenticated")
	}

	friendDiscoveryUUID, err := uuid.Parse(FriendDiscoveryId)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Invalid target user ID %s: %v", FriendDiscoveryId, err)
		return nil, fmt.Errorf("invalid user id")
	}

	query := `
    SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at, gems, xp, all_days_drinking_count, alcoholism_coefficient
    FROM users
    WHERE id = $1
    `
	friendDiscoveryUserData := &user.User{}
	err = s.db.QueryRow(ctx, query, friendDiscoveryUUID).Scan(
		&friendDiscoveryUserData.ID,
		&friendDiscoveryUserData.ClerkID,
		&friendDiscoveryUserData.Email,
		&friendDiscoveryUserData.Username,
		&friendDiscoveryUserData.FirstName,
		&friendDiscoveryUserData.LastName,
		&friendDiscoveryUserData.ImageURL,
		&friendDiscoveryUserData.EmailVerified,
		&friendDiscoveryUserData.CreatedAt,
		&friendDiscoveryUserData.UpdatedAt,
		&friendDiscoveryUserData.Gems,
		&friendDiscoveryUserData.XP,
		&friendDiscoveryUserData.AllDaysDrinkingCount,
		&friendDiscoveryUserData.AlcoholismCoefficient,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("FriendDiscoveryDisplayProfile: User not found for UUID: %s", friendDiscoveryUUID)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("FriendDiscoveryDisplayProfile: Failed to get user: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	friendDiscoveryStats, err := s.GetUserStats(ctx, friendDiscoveryUserData.ClerkID)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Failed to get userStats: %v", err)
		return nil, fmt.Errorf("failed to get userStats: %w", err)
	}

	friendDiscoveryAchievements, err := s.GetAchievements(ctx, friendDiscoveryUserData.ClerkID)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Failed to get user achievements: %v", err)
		return nil, fmt.Errorf("failed to get user achievements: %w", err)
	}

	// ---------------------------------------------------------
	// NEW: Fetch Inventory using the existing GetUserInventory
	// ---------------------------------------------------------
	friendDiscoveryInventory, err := s.GetUserInventory(ctx, friendDiscoveryUserData.ClerkID)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Failed to get user inventory: %v", err)
		// Option A: Return error if inventory is critical
		return nil, fmt.Errorf("failed to get user inventory: %w", err)

		// Option B: If you prefer to return the profile even if inventory fails, un-comment below and comment out the return above:
		// friendDiscoveryInventory = make(map[string][]*store.InventoryItem)
	}

	var isFriend bool
	friendCheckQuery := `
        SELECT EXISTS(
            SELECT 1 FROM friendships
            WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
            AND status = 'accepted'
        )
    `
	err = s.db.QueryRow(ctx, friendCheckQuery, currnetUserID, friendDiscoveryUUID).Scan(&isFriend)
	if err != nil {
		log.Printf("FriendDiscoveryDisplayProfile: Failed to check friendship: %v", err)
		isFriend = false
	}

	userPostsQuery := `
    SELECT 
        dd.id,
        dd.user_id,
        u.image_url AS user_image_url,
		u.username,
        dd.date,
        dd.drank_today,
        dd.logged_at,
        dd.image_url AS post_image_url,
        dd.location_text,
        dd.mentioned_buddies,
        'own' AS source_type
    FROM daily_drinking dd
    JOIN users u ON u.id = dd.user_id
    WHERE dd.user_id = $1
        AND dd.image_url IS NOT NULL
        AND dd.image_url != ''
    ORDER BY dd.logged_at DESC
    `

	rows, err := s.db.Query(ctx, userPostsQuery, friendDiscoveryUUID)
	if err != nil {
		log.Println("failed to get feed")
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	defer rows.Close()

	var userPosts []mix.DailyDrinkingPost
	for rows.Next() {
		var post mix.DailyDrinkingPost
		var mentionedBuddyIDs []string

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
			&post.Username,
			&post.Date,
			&post.DrankToday,
			&post.LoggedAt,
			&post.ImageURL,
			&post.LocationText,
			&mentionedBuddyIDs,
			&post.SourceType,
		)
		if err != nil {
			log.Println("failed to scan post")
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		if len(mentionedBuddyIDs) > 0 {
			post.MentionedBuddies, err = s.getUsersByIDs(ctx, mentionedBuddyIDs)
			if err != nil {
				log.Printf("failed to fetch mentioned buddies for post %s: %v", post.ID, err)
				post.MentionedBuddies = []user.User{}
			}
		} else {
			post.MentionedBuddies = []user.User{}
		}

		userPosts = append(userPosts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating userPosts")
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}

	log.Println("Friend Discovery User Data:", friendDiscoveryUserData)
	log.Println("Friend Discovery Stats:", friendDiscoveryStats)
	log.Println("Friend Discovery Achievements:", friendDiscoveryAchievements)
	log.Println("Is Friend:", isFriend)

	response := &mix.FriendDiscoveryDisplayProfileResponse{
		User:         friendDiscoveryUserData,
		Stats:        friendDiscoveryStats,
		Achievements: friendDiscoveryAchievements,
		MixPosts:     userPosts,
		IsFriend:     isFriend,
		Inventory:    friendDiscoveryInventory,
	}
	return response, nil
}

func (s *UserService) UpdateProfileByClerkID(ctx context.Context, clerkID string, req *user.UpdateProfileRequest) (*user.User, error) {
	query := `
	UPDATE users
	SET 
		username = COALESCE(NULLIF($2, ''), username),
		first_name = COALESCE(NULLIF($3, ''), first_name),
		last_name = COALESCE(NULLIF($4, ''), last_name),
		image_url = COALESCE(NULLIF($5, ''), image_url),
		gems = CASE WHEN $6 != 0 THEN $6 ELSE gems END,
		updated_at = NOW()
	WHERE clerk_id = $1
	RETURNING id, clerk_id, email, username, first_name, last_name, image_url, email_verified, gems, created_at, updated_at
	`

	user := &user.User{}
	err := s.db.QueryRow(
		ctx,
		query,
		clerkID,
		req.Username,
		req.FirstName,
		req.LastName,
		req.ImageURL,
		req.Gems,
	).Scan(
		&user.ID,
		&user.ClerkID,
		&user.Email,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.ImageURL,
		&user.EmailVerified,
		&user.Gems,
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

func (s *UserService) GetFriends(ctx context.Context, clerkID string) ([]*user.User, error) {
	query := `
    SELECT DISTINCT
        u.id,
        u.clerk_id,
        u.email,
        u.username,
        u.first_name,
        u.last_name,
        u.image_url,
        u.email_verified,
        u.created_at,
        u.updated_at
    FROM users u
    INNER JOIN friendships f ON (
        (f.user_id = u.id AND f.friend_id = (SELECT id FROM users WHERE clerk_id = $1))
        OR
        (f.friend_id = u.id AND f.user_id = (SELECT id FROM users WHERE clerk_id = $1))
    )
    WHERE f.status = 'accepted'
    AND u.clerk_id != $1
    ORDER BY u.username
    `

	rows, err := s.db.Query(ctx, query, clerkID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []*user.User
	for rows.Next() {
		var u user.User
		err := rows.Scan(
			&u.ID,
			&u.ClerkID,
			&u.Email,
			&u.Username,
			&u.FirstName,
			&u.LastName,
			&u.ImageURL,
			&u.EmailVerified,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		friends = append(friends, &u)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return friends, nil
}

func (s *UserService) GetDiscovery(ctx context.Context, clerkID string) ([]*user.User, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT
		u.id,
		u.clerk_id,
		u.email,
		u.username,
		u.first_name,
		u.last_name,
		u.image_url,
		u.email_verified,
		u.created_at,
		u.updated_at
	FROM users u
	WHERE u.id != $1
		AND u.id NOT IN (
			-- Exclude existing friends
			SELECT f.friend_id 
			FROM friendships f 
			WHERE f.user_id = $1 AND f.status = 'accepted'
			UNION
			SELECT f.user_id 
			FROM friendships f 
			WHERE f.friend_id = $1 AND f.status = 'accepted'
		)
	ORDER BY RANDOM()
	LIMIT 30
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery users: %w", err)
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		u := &user.User{}
		err := rows.Scan(
			&u.ID,
			&u.ClerkID,
			&u.Email,
			&u.Username,
			&u.FirstName,
			&u.LastName,
			&u.ImageURL,
			&u.EmailVerified,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if users == nil {
		users = []*user.User{}
	}

	return users, nil
}

func (s *UserService) AddFriend(ctx context.Context, clerkID string, friendClerkID string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		log.Printf("AddFriend: Failed to find user with clerk_id %s: %v", clerkID, err)
		return fmt.Errorf("user not found")
	}

	var friendID uuid.UUID
	err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, friendClerkID).Scan(&friendID)
	if err != nil {
		log.Printf("AddFriend: Failed to find friend with clerk_id %s: %v", friendClerkID, err)
		return fmt.Errorf("friend user not found")
	}

	if userID == friendID {
		log.Printf("AddFriend: User %s attempted to add themselves", clerkID)
		return fmt.Errorf("cannot add yourself as a friend")
	}

	var exists bool
	checkQuery := `
		SELECT EXISTS(
			SELECT 1 FROM friendships 
			WHERE (user_id = $1 AND friend_id = $2) 
			   OR (user_id = $2 AND friend_id = $1)
		)
	`
	err = s.db.QueryRow(ctx, checkQuery, userID, friendID).Scan(&exists)
	if err != nil {
		log.Printf("AddFriend: Failed to check existing friendship: %v", err)
		return fmt.Errorf("failed to check existing friendship")
	}

	if exists {
		log.Printf("AddFriend: Friendship already exists between %s and %s", clerkID, friendClerkID)
		return fmt.Errorf("friendship already exists")
	}

	insertQuery := `
		INSERT INTO friendships (user_id, friend_id, status, created_at)
		VALUES ($1, $2, 'accepted', NOW())
	`

	_, err = s.db.Exec(ctx, insertQuery, userID, friendID)
	if err != nil {
		log.Printf("AddFriend: Failed to insert friendship: %v", err)
		return fmt.Errorf("failed to create friendship")
	}

	log.Printf("AddFriend: Successfully created friendship between %s and %s", clerkID, friendClerkID)
	return nil
}

func (s *UserService) RemoveFriend(ctx context.Context, clerkID string, friendClerkID string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		log.Printf("RemoveFriend: Failed to find user with clerk_id %s: %v", clerkID, err)
		return fmt.Errorf("user not found")
	}

	var friendID uuid.UUID
	err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, friendClerkID).Scan(&friendID)
	if err != nil {
		log.Printf("RemoveFriend: Failed to find friend with clerk_id %s: %v", friendClerkID, err)
		return fmt.Errorf("friend user not found")
	}

	deleteQuery := `
		DELETE FROM friendships 
		WHERE (user_id = $1 AND friend_id = $2) 
		   OR (user_id = $2 AND friend_id = $1)
	`

	result, err := s.db.Exec(ctx, deleteQuery, userID, friendID)
	if err != nil {
		log.Printf("RemoveFriend: Failed to delete friendship: %v", err)
		return fmt.Errorf("failed to remove friendship")
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("RemoveFriend: No friendship found between %s and %s", clerkID, friendClerkID)
		return fmt.Errorf("friendship not found")
	}

	log.Printf("RemoveFriend: Successfully removed friendship between %s and %s", clerkID, friendClerkID)
	return nil
}

func (s *UserService) GetLeaderboards(ctx context.Context, clerkID string) (map[string]*leaderboard.Leaderboard, error) {
	// 1. Get YOUR internal UUID (Essential for the rest to work)
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	result := make(map[string]*leaderboard.Leaderboard)

	// --- Global Leaderboard (Standard) ---
	globalQuery := `
		SELECT 
			u.id, u.username, u.image_url,
			COALESCE(u.alcoholism_coefficient, 0) as score,
			RANK() OVER (ORDER BY COALESCE(u.alcoholism_coefficient, 0) DESC) as rank
		FROM users u
		WHERE u.alcoholism_coefficient > 0
		ORDER BY score DESC
		LIMIT 50
	`
	globalRows, err := s.db.Query(ctx, globalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed global: %w", err)
	}
	defer globalRows.Close()

	globalBoard, err := scanLeaderboardRows(globalRows, userID)
	if err != nil {
		return nil, err
	}
	result["global"] = globalBoard

	// --- Friends Leaderboard (THE FIX) ---

	friendsQuery := `
		WITH my_circle AS (
			-- 1. Get IDs where YOU are the 'user_id' (e.g. Row 1, 3 in your screenshot)
			SELECT friend_id AS uid FROM friendships 
			WHERE user_id = $1 AND status = 'accepted'
			
			UNION
			
			-- 2. Get IDs where YOU are the 'friend_id' (e.g. Row 2, 5 in your screenshot)
			SELECT user_id AS uid FROM friendships 
			WHERE friend_id = $1 AND status = 'accepted'
			
			UNION
			
			-- 3. Include YOURSELF so you appear on the leaderboard
			SELECT $1 AS uid
		)
		SELECT 
			u.id,
			u.username,
			u.image_url,
			COALESCE(u.alcoholism_coefficient, 0) as score,
			RANK() OVER (ORDER BY COALESCE(u.alcoholism_coefficient, 0) DESC) as rank
		FROM users u
		INNER JOIN my_circle mc ON u.id = mc.uid
				WHERE u.alcoholism_coefficient > 0
		ORDER BY score DESC
		LIMIT 50
	`

	// CRITICAL: Pass the UUID (userID), NOT the string (clerkID)
	friendsRows, err := s.db.Query(ctx, friendsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch friends leaderboard: %w", err)
	}
	defer friendsRows.Close()

	friendsBoard, err := scanLeaderboardRows(friendsRows, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to scan friends leaderboard: %w", err)
	}
	result["friends"] = friendsBoard

	return result, nil
}

func scanLeaderboardRows(rows pgx.Rows, currentUserID uuid.UUID) (*leaderboard.Leaderboard, error) {
	defer rows.Close()

	var entries []*leaderboard.LeaderboardEntry
	var userPosition *leaderboard.LeaderboardEntry

	for rows.Next() {
		entry := &leaderboard.LeaderboardEntry{}
		err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.ImageURL,
			&entry.AlcoholismCoefficient,
			&entry.Rank,
		)
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)

		if entry.UserID == currentUserID {
			userPosition = entry
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &leaderboard.Leaderboard{
		Entries:      entries,
		UserPosition: userPosition,
		TotalUsers:   len(entries),
	}, nil
}

func (s *UserService) GetAchievements(ctx context.Context, clerkID string) ([]*achievement.AchievementWithStatus, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		a.id,
		a.name,
		a.description,
		a.icon,
		a.criteria_type,
		a.criteria_value,
		a.created_at,
		CASE WHEN ua.id IS NOT NULL THEN true ELSE false END as unlocked,
		ua.unlocked_at
	FROM achievements a
	LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
	ORDER BY unlocked DESC, a.criteria_value ASC
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch achievements: %w", err)
	}
	defer rows.Close()

	var achievements []*achievement.AchievementWithStatus

	for rows.Next() {
		ach := &achievement.AchievementWithStatus{}
		err := rows.Scan(
			&ach.ID,
			&ach.Name,
			&ach.Description,
			&ach.Icon,
			&ach.CriteriaType,
			&ach.CriteriaValue,
			&ach.CreatedAt,
			&ach.Unlocked,
			&ach.UnlockedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan achievement: %w", err)
		}

		achievements = append(achievements, ach)
	}

	return achievements, nil
}

func (s *UserService) AddDrinking(
	ctx context.Context,
	clerkID string,
	drankToday bool,
	imageUrl *string,
	imageWidth *int, // New
	imageHeight *int, // New
	locationText *string,
	lat *float64,
	long *float64,
	alcohols []string,
	clerkIDs []string,
	date time.Time,
) error {
	var userID uuid.UUID
	var username string

	err := s.db.QueryRow(ctx, `SELECT id, username FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID, &username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	var postID uuid.UUID

	// Updated Query to include image_width and image_height
	query := `
        INSERT INTO daily_drinking (
            user_id, 
            date, 
            drank_today, 
            logged_at, 
            image_url, 
            image_width,
            image_height,
            location_text, 
            latitude, 
            longitude, 
            alcohols, 
            mentioned_buddies
        )
        VALUES ($1, $2, $3, NOW(), $4, $10, $11, $5, $6, $7, $8, $9)
        ON CONFLICT (user_id, date) 
        DO UPDATE SET 
            drank_today = $3, 
            logged_at = NOW(), 
            image_url = $4, 
            image_width = $10,
            image_height = $11,
            location_text = $5,
            latitude = $6,
            longitude = $7,
            alcohols = $8,
            mentioned_buddies = $9
        RETURNING id
    `

	err = s.db.QueryRow(ctx, query,
		userID,       // $1
		date,         // $2
		drankToday,   // $3
		imageUrl,     // $4
		locationText, // $5
		lat,          // $6
		long,         // $7
		alcohols,     // $8
		clerkIDs,     // $9
		imageWidth,   // $10
		imageHeight,  // $11
	).Scan(&postID)

	if err != nil {
		return fmt.Errorf("failed to log drinking: %w", err)
	}

	if imageUrl != nil {
		actualURL := *imageUrl
		go utils.FriendPostedImageToMix(s.db, s.notifService, userID, username, actualURL, postID)
	}
	return nil
}

func (s *UserService) GetMemoryWall(ctx context.Context, postIDStr string) ([]canvas.CanvasItem, error) {
	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid post uuid: %w", err)
	}

	query := `
		SELECT 
			id, daily_drinking_id, added_by_user_id, item_type, content,
			pos_x, pos_y, rotation, scale, width, height, z_index, created_at, extra_data
		FROM canvas_items 
		WHERE daily_drinking_id = $1
	`

	rows, err := s.db.Query(ctx, query, postID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var items []canvas.CanvasItem

	for rows.Next() {
		var i canvas.CanvasItem
		var extraDataBytes []byte // Temp holder for JSONB

		err := rows.Scan(
			&i.ID, &i.DailyDrinkingID, &i.AddedByUserID, &i.ItemType, &i.Content,
			&i.PosX, &i.PosY, &i.Rotation, &i.Scale, &i.Width, &i.Height, &i.ZIndex, &i.CreatedAt, &extraDataBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// Convert JSONB bytes back to Map
		if len(extraDataBytes) > 0 {
			if err := json.Unmarshal(extraDataBytes, &i.ExtraData); err != nil {
				return nil, fmt.Errorf("failed to unmarshal extra_data: %w", err)
			}
		}

		// Fetch Author Name/Avatar if needed (Optional: usually easier to just send "Me" or store it,
		// but here we can look it up or leave it nil if the frontend handles it)
		// For now, we leave AuthorName nil and let frontend handle it or do a JOIN in the query above.

		items = append(items, i)
	}

	return items, nil
}

func (s *UserService) AddMemoryToWall(ctx context.Context, clerkID string, postIDStr string, wallItems *[]canvas.CanvasItem, reactions []canvas.CanvasItem) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return fmt.Errorf("invalid post uuid: %w", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	insertQuery := `
		INSERT INTO canvas_items (
			daily_drinking_id, added_by_user_id, item_type, content,
			pos_x, pos_y, rotation, scale, width, height, z_index, extra_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	// ---------------------------------------------------------
	// PART A: HANDLE REACTIONS (Append Only)
	// Strategy: Reactions sent here are NEW.
	// 1. Deduct inventory for all of them.
	// 2. Insert them.
	// ---------------------------------------------------------
	if len(reactions) > 0 {
		reactionCounts := make(map[string]int)

		for _, item := range reactions {
			// 1. Count for Inventory Deduction
			if item.ExtraData != nil {
				if sid, ok := item.ExtraData["inventory_item_id"].(string); ok {
					reactionCounts[sid]++
				} else if sid, ok := item.ExtraData["sticker_id"].(string); ok {
					reactionCounts[sid]++
				}
			}

			// 2. Insert into DB (Force item_type = 'reaction')
			var extraDataJSON []byte
			if item.ExtraData != nil {
				extraDataJSON, _ = json.Marshal(item.ExtraData)
			} else {
				extraDataJSON = []byte("{}")
			}

			_, err := tx.Exec(ctx, insertQuery,
				postID, userID, "reaction", item.Content,
				item.PosX, item.PosY, item.Rotation, item.Scale, item.Width, item.Height, item.ZIndex, extraDataJSON,
			)
			if err != nil {
				return fmt.Errorf("failed to insert reaction: %w", err)
			}
		}

		// 3. Update Inventory (Decrement only)
		for stickerID, count := range reactionCounts {
			if count > 0 {
				_, err := tx.Exec(ctx, `
					UPDATE user_inventory 
					SET quantity = quantity - $1 
					WHERE user_id = $2 AND item_id = $3
				`, count, userID, stickerID)
				if err != nil {
					return fmt.Errorf("failed to deduct inventory for reaction %s: %w", stickerID, err)
				}
			}
		}
	}

	// ---------------------------------------------------------
	// PART B: HANDLE WALL ITEMS (Sync/Edit)
	// Strategy: Only run this if wallItems is NOT nil.
	// This compares 'sticker' types against DB and Syncs them.
	// ---------------------------------------------------------
	if wallItems != nil {
		// 1. Fetch EXISTING 'sticker' items for this user (ignore reactions)
		rows, err := tx.Query(ctx, `
			SELECT extra_data 
			FROM canvas_items 
			WHERE daily_drinking_id = $1 
			AND added_by_user_id = $2
			AND item_type = 'sticker'`, // Only sync stickers, leave reactions alone
			postID, userID,
		)
		if err != nil {
			return fmt.Errorf("failed to fetch existing stickers: %w", err)
		}

		existingCounts := make(map[string]int)
		for rows.Next() {
			var extraDataJSON []byte
			if err := rows.Scan(&extraDataJSON); err == nil && len(extraDataJSON) > 0 {
				var extra map[string]interface{}
				if json.Unmarshal(extraDataJSON, &extra) == nil {
					if sid, ok := extra["inventory_item_id"].(string); ok {
						existingCounts[sid]++
					} else if sid, ok := extra["sticker_id"].(string); ok {
						existingCounts[sid]++
					}
				}
			}
		}
		rows.Close()

		// 2. Count NEW 'sticker' items from payload
		newCounts := make(map[string]int)
		userIDStr := userID.String()
		for _, item := range *wallItems {
			// Only count my own stickers
			isMyItem := item.AddedByUserID == "" || item.AddedByUserID == userIDStr
			if isMyItem && item.ItemType == "sticker" && item.ExtraData != nil {
				if sid, ok := item.ExtraData["inventory_item_id"].(string); ok {
					newCounts[sid]++
				} else if sid, ok := item.ExtraData["sticker_id"].(string); ok {
					newCounts[sid]++
				}
			}
		}

		// 3. Calculate Delta & Update Inventory
		allStickerIDs := make(map[string]bool)
		for k := range existingCounts {
			allStickerIDs[k] = true
		}
		for k := range newCounts {
			allStickerIDs[k] = true
		}

		for stickerID := range allStickerIDs {
			oldQty := existingCounts[stickerID]
			newQty := newCounts[stickerID]
			diff := newQty - oldQty

			if diff != 0 {
				_, err := tx.Exec(ctx, `
					UPDATE user_inventory 
					SET quantity = quantity - $1 
					WHERE user_id = $2 AND item_id = $3
				`, diff, userID, stickerID)
				if err != nil {
					return fmt.Errorf("failed to update inventory for sticker %s: %w", stickerID, err)
				}
			}
		}

		// 4. Delete OLD 'sticker' items (leave reactions alone)
		_, err = tx.Exec(ctx, `
			DELETE FROM canvas_items 
			WHERE daily_drinking_id = $1 
			AND added_by_user_id = $2 
			AND item_type != 'reaction'`, // IMPORTANT: Don't delete reactions during a wall sync
			postID, userID,
		)
		if err != nil {
			return fmt.Errorf("failed to clear old stickers: %w", err)
		}

		// 5. Insert NEW wall items
		for _, item := range *wallItems {
			// Ensure we only insert items marked as mine
			if item.AddedByUserID != "" && item.AddedByUserID != userIDStr {
				continue
			}

			var extraDataJSON []byte
			if item.ExtraData != nil {
				extraDataJSON, _ = json.Marshal(item.ExtraData)
			} else {
				extraDataJSON = []byte("{}")
			}

			_, err := tx.Exec(ctx, insertQuery,
				postID, userID, item.ItemType, item.Content,
				item.PosX, item.PosY, item.Rotation, item.Scale, item.Width, item.Height, item.ZIndex, extraDataJSON,
			)
			if err != nil {
				return fmt.Errorf("failed to insert wall item: %w", err)
			}
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	//! send to post owner that someone reacted to his mix post
	// go utils.ReactionToPostMix(s.db, s.notifService, )
	return err
}
func (s *UserService) AddMixVideo(ctx context.Context, clerkID string, videoUrl string, caption *string, duration int) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
        INSERT INTO mix_videos (user_id, video_url, caption, duration, chips)
        VALUES ($1, $2, $3, $4, 0)
    
		  `

	_, err = s.db.Exec(ctx, query, userID, videoUrl, caption, duration)
	if err != nil {
		return fmt.Errorf("failed to insert mix video: %w", err)
	}

	return nil
}

func (s *UserService) AddUserFeedback(ctx context.Context, clerkID string, category string, feedbackText string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
        INSERT INTO feedback (user_id, category, feedback_text)
        VALUES ($1, $2, $3)
    `

	_, err = s.db.Exec(ctx, query, userID, category, feedbackText)
	if err != nil {
		return fmt.Errorf("failed to insert feedback: %w", err)
	}

	return nil
}

func (s *UserService) RemoveDrinking(ctx context.Context, clerkID string, date time.Time) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
        DELETE FROM daily_drinking 
        WHERE user_id = $1 AND date = $2
    `

	result, err := s.db.Exec(ctx, query, userID, date)
	if err != nil {
		return fmt.Errorf("failed to remove drinking log: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no drinking log found for the specified date")
	}

	return nil
}

func (s *UserService) GetDrunkThought(ctx context.Context, clerkID string, date time.Time) (*string, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	var drunkThought *string

	query := `
		SELECT drunk_thought
		FROM daily_drinking
		WHERE user_id = $1 AND date = $2
	`

	err = s.db.QueryRow(ctx, query, userID, date).Scan(&drunkThought)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get drunk thought: %w", err)
	}

	return drunkThought, nil
}

func (s *UserService) AddDrunkThought(ctx context.Context, clerkID string, drunkThought string) (*string, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	var savedThought string
	query := `
        INSERT INTO daily_drinking (user_id, date, drunk_thought)
        VALUES ($1, CURRENT_DATE, $2)
        ON CONFLICT (user_id, date) 
        DO UPDATE SET 
            drunk_thought = $2
        RETURNING drunk_thought
    `
	err = s.db.QueryRow(ctx, query, userID, drunkThought).Scan(&savedThought)
	if err != nil {
		return nil, fmt.Errorf("failed to log drunk thought: %w", err)
	}

	return &savedThought, nil
}

func (s *UserService) GetWeeklyDaysDrank(ctx context.Context, clerkID string) (*stats.DaysStat, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT COALESCE(COUNT(*) FILTER (WHERE drank_today = true), 0) as days_drank
	FROM daily_drinking
	WHERE user_id = $1
		AND date >= DATE_TRUNC('week', CURRENT_DATE)
		AND date <= CURRENT_DATE
	`

	stat := &stats.DaysStat{Period: "week", TotalDays: 7}
	err = s.db.QueryRow(ctx, query, userID).Scan(&stat.DaysDrank)
	if err != nil {
		return nil, fmt.Errorf("failed to get weekly stats: %w", err)
	}

	return stat, nil
}

func (s *UserService) SearchUsers(ctx context.Context, clerkID string, query string) ([]*user.User, error) {
	cleanQuery := strings.TrimSpace(query)
	searchPattern := "%" + cleanQuery + "%"
	startsWithPattern := cleanQuery + "%"

	sqlQuery := `
	SELECT 
		id, 
		clerk_id, 
		email, 
		username, 
		first_name, 
		last_name, 
		image_url, 
		email_verified, 
		created_at, 
		updated_at,
		similarity_score
	FROM (
		SELECT 
			id, 
			clerk_id, 
			email, 
			username, 
			first_name, 
			last_name, 
			image_url, 
			email_verified, 
			created_at, 
			updated_at,
			-- Calculate similarity score (0-100%)
			GREATEST(
				-- Exact match (100%)
				CASE 
					WHEN LOWER(username) = LOWER($2) THEN 100
					WHEN LOWER(email) = LOWER($2) THEN 100
					WHEN LOWER(first_name) = LOWER($2) THEN 95
					WHEN LOWER(last_name) = LOWER($2) THEN 95
					WHEN LOWER(CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, ''))) = LOWER($2) THEN 100
					ELSE 0
				END,
				-- Starts with match (80-90%)
				CASE 
					WHEN LOWER(username) LIKE LOWER($3) THEN 90
					WHEN LOWER(first_name) LIKE LOWER($3) THEN 85
					WHEN LOWER(last_name) LIKE LOWER($3) THEN 85
					WHEN LOWER(CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, ''))) LIKE LOWER($3) THEN 88
					ELSE 0
				END,
				-- Contains match (50-70%)
				CASE 
					WHEN LOWER(username) LIKE LOWER($1) THEN 70
					WHEN LOWER(first_name) LIKE LOWER($1) THEN 60
					WHEN LOWER(last_name) LIKE LOWER($1) THEN 60
					WHEN LOWER(CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, ''))) LIKE LOWER($1) THEN 65
					WHEN LOWER(email) LIKE LOWER($1) THEN 50
					ELSE 0
				END
			) AS similarity_score
		FROM users
		WHERE 
			(
				username ILIKE $1 OR
				email ILIKE $1 OR
				first_name ILIKE $1 OR
				last_name ILIKE $1 OR
				CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, '')) ILIKE $1
			)
			-- Exclude the searching user
			AND clerk_id != $4
	) AS scored_users
	WHERE similarity_score >= 30
	ORDER BY 
		similarity_score DESC,
		username
	LIMIT 50
	`

	rows, err := s.db.Query(ctx, sqlQuery, searchPattern, cleanQuery, startsWithPattern, clerkID)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		u := &user.User{}
		var similarityScore float64

		err := rows.Scan(
			&u.ID,
			&u.ClerkID,
			&u.Email,
			&u.Username,
			&u.FirstName,
			&u.LastName,
			&u.ImageURL,
			&u.EmailVerified,
			&u.CreatedAt,
			&u.UpdatedAt,
			&similarityScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if users == nil {
		users = []*user.User{}
	}

	return users, nil
}

func (s *UserService) GetMonthlyDaysDrank(ctx context.Context, clerkID string) (*stats.DaysStat, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT COALESCE(COUNT(*) FILTER (WHERE drank_today = true), 0) as days_drank
	FROM daily_drinking
	WHERE user_id = $1
		AND date >= DATE_TRUNC('month', CURRENT_DATE)
		AND date <= CURRENT_DATE
	`

	daysInMonth := time.Now().AddDate(0, 1, -time.Now().Day()).Day()
	stat := &stats.DaysStat{Period: "month", TotalDays: daysInMonth}
	err = s.db.QueryRow(ctx, query, userID).Scan(&stat.DaysDrank)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly stats: %w", err)
	}

	return stat, nil
}

func (s *UserService) GetYearlyDaysDrank(ctx context.Context, clerkID string) (*stats.DaysStat, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT COALESCE(COUNT(*) FILTER (WHERE drank_today = true), 0) as days_drank
	FROM daily_drinking
	WHERE user_id = $1
		AND date >= DATE_TRUNC('year', CURRENT_DATE)
		AND date <= CURRENT_DATE
	`

	now := time.Now()
	daysInYear := 365
	if now.Year()%4 == 0 && (now.Year()%100 != 0 || now.Year()%400 == 0) {
		daysInYear = 366
	}

	stat := &stats.DaysStat{Period: "year", TotalDays: daysInYear}
	err = s.db.QueryRow(ctx, query, userID).Scan(&stat.DaysDrank)
	if err != nil {
		return nil, fmt.Errorf("failed to get yearly stats: %w", err)
	}

	return stat, nil
}

func (s *UserService) GetAllTimeDaysDrank(ctx context.Context, clerkID string) (*stats.DaysStat, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		COALESCE(COUNT(*) FILTER (WHERE drank_today = true), 0) as days_drank,
		COALESCE(COUNT(DISTINCT date), 0) as total_days
	FROM daily_drinking
	WHERE user_id = $1
	`

	stat := &stats.DaysStat{Period: "all_time"}
	err = s.db.QueryRow(ctx, query, userID).Scan(&stat.DaysDrank, &stat.TotalDays)
	if err != nil {
		return nil, fmt.Errorf("failed to get all time stats: %w", err)
	}

	return stat, nil
}

func (s *UserService) GetCalendar(ctx context.Context, clerkID string, year int, month int, displyUserId *string) (*calendar.CalendarResponse, error) {
	var targetUserID uuid.UUID
	var err error
	log.Println("------------------------------------------------")
	log.Println(displyUserId)

	// 1. Determine which User ID to fetch
	if displyUserId != nil && *displyUserId != "" {
		// Case A: A specific user ID was provided via query param
		targetUserID, err = uuid.Parse(*displyUserId)
		if err != nil {
			return nil, fmt.Errorf("invalid display user id format: %w", err)
		}
	} else {
		// Case B: No specific user provided, look up the authenticated user via Clerk ID
		err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&targetUserID)
		if err != nil {
			return nil, fmt.Errorf("current user not found: %w", err)
		}
	}

	// 2. Setup Date Range
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	// 3. Query using the determined targetUserID
	query := `
	SELECT date, drank_today
	FROM daily_drinking
	WHERE user_id = $1
		AND date >= $2
		AND date <= $3
	ORDER BY date
	`

	rows, err := s.db.Query(ctx, query, targetUserID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar: %w", err)
	}
	defer rows.Close()

	// 4. Map Results
	dayMap := make(map[string]bool)
	for rows.Next() {
		var date time.Time
		var drank bool
		if err := rows.Scan(&date, &drank); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		dayMap[date.Format("2006-01-02")] = drank
	}

	// 5. Build Calendar Response
	var days []*calendar.CalendarDay
	today := time.Now().Format("2006-01-02")

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		day := &calendar.CalendarDay{
			Date:       d,
			DrankToday: dayMap[dateStr],
			IsToday:    dateStr == today,
		}
		days = append(days, day)
	}

	return &calendar.CalendarResponse{
		Year:  year,
		Month: month,
		Days:  days,
	}, nil
}

func (s *UserService) GetUserStats(ctx context.Context, clerkID string) (*stats.UserStats, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
    WITH RECURSIVE streak_calc AS (
    -- Start with today or yesterday ONLY if they drank
    SELECT 
        user_id,
        date,
        1 as streak_length
    FROM daily_drinking
    WHERE user_id = $1 
        AND drank_today = true
        AND date = (
            SELECT MAX(date) 
            FROM daily_drinking 
            WHERE user_id = $1 
                AND drank_today = true 
                AND date <= CURRENT_DATE
        )
        -- KEY FIX: Only start if the most recent day is today or yesterday
        AND date >= CURRENT_DATE - INTERVAL '1 day'
    
    UNION ALL
    
    -- Recursively check previous days
    SELECT 
        dd.user_id,
        dd.date,
        sc.streak_length + 1
    FROM daily_drinking dd
    INNER JOIN streak_calc sc ON dd.user_id = sc.user_id 
        AND dd.date = sc.date - INTERVAL '1 day'
    WHERE dd.drank_today = true
),
    current_streak_result AS (
        SELECT 
            user_id,
            MAX(streak_length) as current_streak
        FROM streak_calc
        GROUP BY user_id
    ),
    longest_streak_calc AS (
        SELECT 
            user_id,
            MAX(streak_length) as longest_streak
        FROM (
            SELECT 
                user_id,
                COUNT(*) as streak_length
            FROM (
                SELECT 
                    user_id,
                    date,
                    date - (ROW_NUMBER() OVER (ORDER BY date))::int AS grp
                FROM daily_drinking
                WHERE user_id = $1 AND drank_today = true
            ) sub
            GROUP BY user_id, grp
        ) streaks
        GROUP BY user_id
    )
    SELECT 
        COALESCE(dd_today.drank_today, false) as today_status,
        COALESCE(COUNT(DISTINCT dd_week.date) FILTER (WHERE dd_week.drank_today = true), 0) as days_this_week,
        COALESCE(COUNT(DISTINCT dd_month.date) FILTER (WHERE dd_month.drank_today = true), 0) as days_this_month,
        COALESCE(COUNT(DISTINCT dd_year.date) FILTER (WHERE dd_year.drank_today = true), 0) as days_this_year,
        COALESCE(COUNT(DISTINCT dd_all.date) FILTER (WHERE dd_all.drank_today = true), 0) as total_days_drank,
        COALESCE(sc.current_streak, 0) as current_streak,
        COALESCE(lsc.longest_streak, 0) as longest_streak,
        COALESCE(SUM(ws.win_count), 0) as total_weeks_won,
        COUNT(DISTINCT ua.achievement_id) as achievements_count,
        COUNT(DISTINCT f.friend_id) FILTER (WHERE f.status = 'accepted') as friends_count
    FROM users u
    LEFT JOIN daily_drinking dd_today ON u.id = dd_today.user_id AND dd_today.date = CURRENT_DATE
    LEFT JOIN daily_drinking dd_week ON u.id = dd_week.user_id 
        AND dd_week.date >= DATE_TRUNC('week', CURRENT_DATE)
        AND dd_week.date <= CURRENT_DATE
    LEFT JOIN daily_drinking dd_month ON u.id = dd_month.user_id 
        AND dd_month.date >= DATE_TRUNC('month', CURRENT_DATE)
        AND dd_month.date <= CURRENT_DATE
    LEFT JOIN daily_drinking dd_year ON u.id = dd_year.user_id 
        AND dd_year.date >= DATE_TRUNC('year', CURRENT_DATE)
        AND dd_year.date <= CURRENT_DATE
    LEFT JOIN daily_drinking dd_all ON u.id = dd_all.user_id
    LEFT JOIN current_streak_result sc ON u.id = sc.user_id
    LEFT JOIN longest_streak_calc lsc ON u.id = lsc.user_id
    LEFT JOIN weekly_stats ws ON u.id = ws.user_id
    LEFT JOIN user_achievements ua ON u.id = ua.user_id
    LEFT JOIN friendships f ON u.id = f.user_id
    WHERE u.id = $1
    GROUP BY u.id, dd_today.drank_today, sc.current_streak, lsc.longest_streak
    `

	stats := &stats.UserStats{}
	err = s.db.QueryRow(ctx, query, userID).Scan(
		&stats.TodayStatus,
		&stats.DaysThisWeek,
		&stats.DaysThisMonth,
		&stats.DaysThisYear,
		&stats.TotalDaysDrank,
		&stats.CurrentStreak,
		&stats.LongestStreak,
		&stats.TotalWeeksWon,
		&stats.AchievementsCount,
		&stats.FriendsCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	stats.AlcoholismCoefficient = utils.CalculateAlcoholismScore(
		stats.CurrentStreak,
		stats.TotalDaysDrank,
		stats.AchievementsCount,
	)

	_, err = s.db.Exec(ctx, `
		UPDATE users 
		SET alcoholism_coefficient = $1 
		WHERE id = $2
	`, stats.AlcoholismCoefficient, userID)

	if err != nil {
		fmt.Printf("failed to update alcoholism coefficient: %v\n", err)
	}

	rankQuery := `
    WITH user_scores AS (
        SELECT 
            u.id,
            COALESCE(
                (SELECT MAX(streak_length) 
                 FROM (
                     WITH RECURSIVE streak_calc AS (
                         SELECT 
                             user_id,
                             date,
                             1 as streak_length
                         FROM daily_drinking
                         WHERE user_id = u.id 
                             AND drank_today = true
                             AND date = (
                                 SELECT MAX(date) 
                                 FROM daily_drinking 
                                 WHERE user_id = u.id 
                                     AND drank_today = true 
                                     AND date <= CURRENT_DATE
                             )
                         UNION ALL
                         SELECT 
                             dd.user_id,
                             dd.date,
                             sc.streak_length + 1
                         FROM daily_drinking dd
                         INNER JOIN streak_calc sc ON dd.user_id = sc.user_id 
                             AND dd.date = sc.date - INTERVAL '1 day'
                         WHERE dd.drank_today = true
                     )
                     SELECT streak_length FROM streak_calc
                 ) s
                ), 0
            ) as current_streak,
            COALESCE(COUNT(DISTINCT dd.date) FILTER (WHERE dd.drank_today = true), 0) as total_days,
            COALESCE(COUNT(DISTINCT ua.achievement_id), 0) as achievements
        FROM users u
        LEFT JOIN daily_drinking dd ON u.id = dd.user_id
        LEFT JOIN user_achievements ua ON u.id = ua.user_id
        GROUP BY u.id
    ),
    ranked_users AS (
        SELECT 
            id,
            (current_streak * current_streak * 0.3) + (total_days * 0.05) + (achievements * 1.0) as score,
            RANK() OVER (ORDER BY (current_streak * current_streak * 0.3) + (total_days * 0.05) + (achievements * 1.0) DESC) as rank
        FROM user_scores
    )
    SELECT rank
    FROM ranked_users
    WHERE id = $1
    `

	err = s.db.QueryRow(ctx, rankQuery, userID).Scan(&stats.Rank)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate rank: %w", err)
	}

	return stats, nil
}

func (s *UserService) GetYourMix(ctx context.Context, clerkID string, page int, limit int) ([]mix.DailyDrinkingPost, error) {
	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	offset := (page - 1) * limit

	query := `
    SELECT 
        dd.id,
        dd.user_id,
        u.image_url AS user_image_url,
        u.username,
        dd.date,
        dd.drank_today,
        dd.logged_at,
        dd.image_url AS post_image_url,
		COALESCE(dd.image_width, 0),   
        COALESCE(dd.image_height, 0),
        dd.location_text,
        dd.mentioned_buddies,
        CASE 
            WHEN dd.user_id = $1 THEN 'me' 
            ELSE 'friend' 
        END AS source_type,
        -- AGGREGATE REACTIONS HERE
        COALESCE(
            (
                SELECT json_agg(json_build_object(
                    'id', ci.id,
                    'item_type', ci.item_type,
                    'content', ci.content,
                    'pos_x', ci.pos_x,
                    'pos_y', ci.pos_y,
                    'rotation', ci.rotation,
                    'scale', ci.scale,
                    'width', ci.width,
                    'height', ci.height,
                    'z_index', ci.z_index,
                    'extra_data', ci.extra_data
                ))
                FROM canvas_items ci
                WHERE ci.daily_drinking_id = dd.id 
                AND ci.item_type = 'reaction' -- Only fetch items marked as reactions
            ), 
            '[]'::json
        ) AS reactions
    FROM daily_drinking dd
    JOIN users u ON u.id = dd.user_id
    WHERE 
        dd.image_url IS NOT NULL 
        AND dd.image_url != ''
        AND (
            -- Include Self
            dd.user_id = $1
            OR 
            -- Include Friends (Bidirectional)
            dd.user_id IN (
                SELECT friend_id FROM friendships WHERE user_id = $1 AND status = 'accepted'
                UNION
                SELECT user_id FROM friendships WHERE friend_id = $1 AND status = 'accepted'
            )
        )
    ORDER BY dd.logged_at DESC
    LIMIT $2 OFFSET $3
    `

	return s.executeFeedQuery(ctx, query, userID, limit, offset)
}

func (s *UserService) GetGlobalMix(ctx context.Context, clerkID string, page int, limit int) ([]mix.DailyDrinkingPost, error) {
	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	offset := (page - 1) * limit

	query := `
    SELECT 
        dd.id,
        dd.user_id,
        u.image_url AS user_image_url,
        u.username,
        dd.date,
        dd.drank_today,
        dd.logged_at,
        dd.image_url AS post_image_url,
		COALESCE(dd.image_width, 0),  
        COALESCE(dd.image_height, 0),
        dd.location_text,
        dd.mentioned_buddies,
        'other' AS source_type,
        -- AGGREGATE REACTIONS
        COALESCE(
            (
                SELECT json_agg(json_build_object(
                    'id', ci.id,
                    'item_type', ci.item_type,
                    'content', ci.content,
                    'pos_x', ci.pos_x,
                    'pos_y', ci.pos_y,
                    'rotation', ci.rotation,
                    'scale', ci.scale,
                    'width', ci.width,
                    'height', ci.height,
                    'z_index', ci.z_index,
                    'extra_data', ci.extra_data
                ))
                FROM canvas_items ci
                WHERE ci.daily_drinking_id = dd.id 
                AND ci.item_type = 'reaction'
            ), 
            '[]'::json
        ) AS reactions
    FROM daily_drinking dd
    JOIN users u ON u.id = dd.user_id
    WHERE dd.user_id != $1
        AND dd.image_url IS NOT NULL 
        AND dd.image_url != ''
        AND dd.user_id NOT IN (
            -- Exclude Friends
            SELECT friend_id FROM friendships WHERE user_id = $1 AND status = 'accepted'
            UNION
            SELECT user_id FROM friendships WHERE friend_id = $1 AND status = 'accepted'
        )
    ORDER BY dd.logged_at DESC
    LIMIT $2 OFFSET $3
    `

	return s.executeFeedQuery(ctx, query, userID, limit, offset)
}

func (s *UserService) executeFeedQuery(ctx context.Context, query string, userID string, limit, offset int) ([]mix.DailyDrinkingPost, error) {
	rows, err := s.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		log.Println("failed to get feed")
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	defer rows.Close()

	var posts []mix.DailyDrinkingPost
	for rows.Next() {
		var post mix.DailyDrinkingPost
		var mentionedBuddyIDs []string
		var reactionsJSON []byte

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
			&post.Username,
			&post.Date,
			&post.DrankToday,
			&post.LoggedAt,
			&post.ImageURL,
			&post.ImageWidth,
			&post.ImageHeight,
			&post.LocationText,
			&mentionedBuddyIDs,
			&post.SourceType,
			&reactionsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		// 3. Unmarshal Reactions
		if len(reactionsJSON) > 0 {
			if err := json.Unmarshal(reactionsJSON, &post.Reactions); err != nil {
				log.Printf("failed to unmarshal reactions for post %s: %v", post.ID, err)
				post.Reactions = []canvas.CanvasItem{}
			}
		} else {
			post.Reactions = []canvas.CanvasItem{}
		}

		// Hydrate Buddies
		if len(mentionedBuddyIDs) > 0 {
			post.MentionedBuddies, err = s.getUsersByIDs(ctx, mentionedBuddyIDs)
			if err != nil {
				// Log error but don't fail the whole feed
				log.Printf("failed to fetch buddies for post %s: %v", post.ID, err)
				post.MentionedBuddies = []user.User{}
			}
		} else {
			post.MentionedBuddies = []user.User{}
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}

	if posts == nil {
		posts = []mix.DailyDrinkingPost{}
	}

	return posts, nil
}

func (s *UserService) GetUserFriendsPosts(ctx context.Context, clerkID string) ([]mix.DailyDrinkingPost, error) {
	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
		SELECT 
			dd.id,
			dd.user_id,
			u.image_url AS user_image_url,
			u.username,
			dd.date,
			dd.drank_today,
			dd.logged_at,
			COALESCE(dd.image_url, '') AS post_image_url, -- Use COALESCE to safely handle NULLs
			dd.location_text,
			dd.latitude,    
			dd.longitude,   
			dd.alcohols,    
			dd.mentioned_buddies,
			CASE 
				WHEN dd.user_id = $1 THEN 'self' 
				ELSE 'friend' 
			END AS source_type
		FROM daily_drinking dd
		JOIN users u ON u.id = dd.user_id
		WHERE 
			dd.latitude IS NOT NULL     -- We still keep location checks for the map
			AND dd.longitude IS NOT NULL
			AND (
				-- Include the user's own posts
				dd.user_id = $1
				OR 
				-- Include friends' posts
				dd.user_id IN (
					SELECT friend_id FROM friendships 
					WHERE user_id = $1 AND status = 'accepted'
					UNION
					SELECT user_id FROM friendships 
					WHERE friend_id = $1 AND status = 'accepted'
				)
			)
		ORDER BY dd.logged_at DESC
		LIMIT 200
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get feed")
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	defer rows.Close()

	var posts []mix.DailyDrinkingPost
	for rows.Next() {
		var post mix.DailyDrinkingPost
		var mentionedBuddyIDs []string

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
			&post.Username,
			&post.Date,
			&post.DrankToday,
			&post.LoggedAt,
			&post.ImageURL,
			&post.LocationText,
			&post.Latitude,
			&post.Longitude,
			&post.Alcohols,
			&mentionedBuddyIDs,
			&post.SourceType,
		)
		if err != nil {
			log.Println("failed to scan post", err)
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		if len(mentionedBuddyIDs) > 0 {
			post.MentionedBuddies, err = s.getUsersByIDs(ctx, mentionedBuddyIDs)
			if err != nil {
				log.Printf("failed to fetch mentioned buddies for post %s: %v", post.ID, err)
				post.MentionedBuddies = []user.User{}
			}
		} else {
			post.MentionedBuddies = []user.User{}
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}

	return posts, nil
}

func (s *UserService) GetMixVideoFeed(ctx context.Context, clerkID string) ([]mix.VideoPost, error) {
	log.Println("getting video feed")

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	log.Printf("Found user ID: %s for clerk_id: %s", userID, clerkID)

	query := `
	WITH friend_videos AS (
		-- Get videos from friends (up to 30 videos = 60% of 50)
		SELECT 
			mv.id,
			mv.user_id,
			u.username,
			u.image_url AS user_image_url,
			mv.video_url,
			mv.caption,
			mv.chips,
			mv.duration,
			mv.created_at,
			'friend' AS source_type
		FROM mix_videos mv
		JOIN users u ON u.id = mv.user_id
		WHERE mv.user_id != $1
			AND mv.created_at >= NOW() - INTERVAL '7 days'
			AND mv.user_id IN (
				-- Get all friends (bidirectional)
				SELECT friend_id FROM friendships 
				WHERE user_id = $1 AND status = 'accepted'
				UNION
				SELECT user_id FROM friendships 
				WHERE friend_id = $1 AND status = 'accepted'
			)
		ORDER BY mv.created_at DESC
		LIMIT 30
	),
	friend_count AS (
		-- Count how many friend videos we got
		SELECT COUNT(*) as cnt FROM friend_videos
	),
	other_videos AS (
		-- Get videos from non-friends
		-- Calculate limit: 50 - friend_count, with minimum of 20 (40% of 50)
		SELECT 
			mv.id,
			mv.user_id,
			u.username,
			u.image_url AS user_image_url,
			mv.video_url,
			mv.caption,
			mv.chips,
			mv.duration,
			mv.created_at,
			'other' AS source_type
		FROM mix_videos mv
		JOIN users u ON u.id = mv.user_id
		WHERE mv.user_id != $1
			AND mv.created_at >= NOW() - INTERVAL '7 days'
			AND mv.user_id NOT IN (
				-- Exclude friends (bidirectional)
				SELECT friend_id FROM friendships 
				WHERE user_id = $1 AND status = 'accepted'
				UNION
				SELECT user_id FROM friendships 
				WHERE friend_id = $1 AND status = 'accepted'
			)
		ORDER BY mv.created_at DESC
		LIMIT GREATEST(20, 50 - (SELECT cnt FROM friend_count))
	)
	-- Combine and return final feed
	SELECT 
		id,
		user_id,
		username,
		user_image_url,
		video_url,
		caption,
		chips,
		duration,
		created_at
	FROM (
		SELECT * FROM friend_videos
		UNION ALL
		SELECT * FROM other_videos
	) AS combined_feed
	ORDER BY created_at DESC
	LIMIT 50
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get video feed")
		return nil, fmt.Errorf("failed to get video feed: %w", err)
	}
	defer rows.Close()

	var videos []mix.VideoPost
	for rows.Next() {
		var video mix.VideoPost

		err := rows.Scan(
			&video.ID,
			&video.UserID,
			&video.Username,
			&video.UserImageUrl,
			&video.VideoUrl,
			&video.Caption,
			&video.Chips,
			&video.Duration,
			&video.CreatedAt,
		)
		if err != nil {
			log.Println("failed to scan video")
			return nil, fmt.Errorf("failed to scan video: %w", err)
		}

		videos = append(videos, video)
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating videos")
		return nil, fmt.Errorf("error iterating videos: %w", err)
	}
	log.Printf("Returning %d videos", len(videos))

	return videos, nil
}

func (s *UserService) GetMixTimeline(ctx context.Context, clerkID string) ([]mix.DailyDrinkingPost, error) {
	log.Println("getting user mix timeline")

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		dd.id,
		dd.user_id,
		u.image_url AS user_image_url,
		u.username,
		dd.date,
		dd.drank_today,
		dd.logged_at,
		dd.image_url AS post_image_url,
		dd.location_text,
		dd.mentioned_buddies,
		'own' AS source_type
	FROM daily_drinking dd
	JOIN users u ON u.id = dd.user_id
	WHERE dd.user_id = $1
		AND dd.image_url IS NOT NULL
		AND dd.image_url != ''
	ORDER BY dd.logged_at DESC
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get feed")
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	defer rows.Close()

	var posts []mix.DailyDrinkingPost
	for rows.Next() {
		var post mix.DailyDrinkingPost
		var mentionedBuddyIDs []string

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
			&post.Username,
			&post.Date,
			&post.DrankToday,
			&post.LoggedAt,
			&post.ImageURL,
			&post.LocationText,
			&mentionedBuddyIDs,
			&post.SourceType,
		)
		if err != nil {
			log.Println("failed to scan post")
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		if len(mentionedBuddyIDs) > 0 {
			post.MentionedBuddies, err = s.getUsersByIDs(ctx, mentionedBuddyIDs)
			if err != nil {
				log.Printf("failed to fetch mentioned buddies for post %s: %v", post.ID, err)
				// Continue without buddies rather than failing
				post.MentionedBuddies = []user.User{}
			}
		} else {
			post.MentionedBuddies = []user.User{}
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating posts")
		return nil, fmt.Errorf("error iterating posts: %w", err)
	}
	log.Println(posts)

	return posts, nil
}

func (s *UserService) AddChipsToVideo(ctx context.Context, clerkID string, videoID string) error {
	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	_, err = s.db.Exec(ctx,
		"INSERT INTO mix_video_likes (video_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		videoID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to like video: %w", err)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE mix_videos SET chips = COALESCE(chips, 0) + 1 WHERE id = $1",
		videoID,
	)
	if err != nil {
		return fmt.Errorf("failed to increment chips: %w", err)
	}

	return nil
}

func (s *UserService) getUsersByIDs(ctx context.Context, clerkIDs []string) ([]user.User, error) {
	if len(clerkIDs) == 0 {
		return []user.User{}, nil
	}

	query := `
		SELECT 
			id,
			clerk_id,
			username,
			email,
			first_name,
			last_name,
			image_url,
			created_at,
			updated_at,
			gems,
			xp,
			all_days_drinking_count
		FROM users
		WHERE clerk_id = ANY($1)
	`

	rows, err := s.db.Query(ctx, query, clerkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []user.User
	for rows.Next() {
		var u user.User
		err := rows.Scan(
			&u.ID,
			&u.ClerkID,
			&u.Username,
			&u.Email,
			&u.FirstName,
			&u.LastName,
			&u.ImageURL,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.Gems,
			&u.XP,
			&u.AllDaysDrinkingCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

func (s *UserService) GetDrunkFriendThoughts(ctx context.Context, clerkID string) ([]user.DrunkThought, error) {
	log.Println("getting drunk friends thoughts")

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
    SELECT 
        dd.id,
        dd.user_id,
        u.username,
        u.image_url,
        dd.drunk_thought,
        dd.logged_at
    FROM daily_drinking dd
    JOIN users u ON u.id = dd.user_id
    JOIN friendships f ON (
        (f.user_id = $1 AND f.friend_id = dd.user_id) OR 
        (f.friend_id = $1 AND f.user_id = dd.user_id)
    )
    WHERE f.status = 'accepted'
        AND dd.drunk_thought IS NOT NULL
        AND dd.drunk_thought != ''
        AND dd.date >= CURRENT_DATE - INTERVAL '7 days'
    ORDER BY dd.logged_at DESC
    `

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get drunk friend thoughts:", err)
		return nil, fmt.Errorf("failed to get drunk friend thoughts: %w", err)
	}
	defer rows.Close()

	var thoughts []user.DrunkThought
	for rows.Next() {
		var thought user.DrunkThought
		err := rows.Scan(
			&thought.ID,
			&thought.UserID,
			&thought.Username,
			&thought.UserImageURL,
			&thought.Thought,
			&thought.CreatedAt,
		)
		if err != nil {
			log.Println("failed to scan thought:", err)
			return nil, fmt.Errorf("failed to scan thought: %w", err)
		}

		thoughts = append(thoughts, thought)
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating thoughts:", err)
		return nil, fmt.Errorf("error iterating thoughts: %w", err)
	}

	log.Printf("found %d drunk thoughts from friends", len(thoughts))
	return thoughts, nil
}

func (s *UserService) GetAlcoholCollection(ctx context.Context, clerkID string) ([]user.DrunkThought, error) {
	log.Println("getting alcohol collection")

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
    SELECT 
        collection_data.name,
        collection_data.type,
        u.username,
        u.image_url,
        dd.drunk_thought,
        dd.logged_at
    FROM db_alcohol_collection_data collection_data
    JOIN users u ON u.id = dd.user_id
    JOIN friendships f ON (
        (f.user_id = $1 AND f.friend_id = dd.user_id) OR 
        (f.friend_id = $1 AND f.user_id = dd.user_id)
    )
    WHERE f.status = 'accepted'
        AND dd.drunk_thought IS NOT NULL
        AND dd.drunk_thought != ''
        AND dd.date >= CURRENT_DATE - INTERVAL '7 days'
    ORDER BY dd.logged_at DESC
    `

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get drunk friend thoughts:", err)
		return nil, fmt.Errorf("failed to get drunk friend thoughts: %w", err)
	}
	defer rows.Close()

	var thoughts []user.DrunkThought
	for rows.Next() {
		var thought user.DrunkThought
		err := rows.Scan(
			&thought.ID,
			&thought.UserID,
			&thought.Username,
			&thought.UserImageURL,
			&thought.Thought,
			&thought.CreatedAt,
		)
		if err != nil {
			log.Println("failed to scan thought:", err)
			return nil, fmt.Errorf("failed to scan thought: %w", err)
		}

		thoughts = append(thoughts, thought)
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating thoughts:", err)
		return nil, fmt.Errorf("error iterating thoughts: %w", err)
	}

	log.Printf("found %d drunk thoughts from friends", len(thoughts))
	return thoughts, nil
}

func (s *UserService) SearchDbAlcohol(ctx context.Context, clerkID string, queryAlcoholName string) (map[string]interface{}, error) {
	log.Println("searching alcohol collection")

	var userID string
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		log.Println("failed to get user ID:", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	cleanQuery := strings.TrimSpace(queryAlcoholName)
	searchPattern := "%" + cleanQuery + "%"
	startsWithPattern := cleanQuery + "%"

	query := `
    SELECT 
        id,
        name,
        type,
        image_url,
        rarity,
        abv
    FROM db_alcohol_collection_data
    WHERE LOWER(name) LIKE LOWER($1)
    ORDER BY
        CASE
            WHEN LOWER(name) = LOWER($2) THEN 100
            WHEN LOWER(name) LIKE LOWER($3) THEN 90
            WHEN LOWER(name) LIKE LOWER($1) THEN 80
            ELSE 0
        END DESC
    LIMIT 1
    `

	var item collection.AlcoholItem
	err = s.db.QueryRow(ctx, query, searchPattern, cleanQuery, startsWithPattern).Scan(
		&item.ID,
		&item.Name,
		&item.Type,
		&item.ImageUrl,
		&item.Rarity,
		&item.Abv,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			log.Println("no alcohol found matching query:", queryAlcoholName)
			return nil, nil
		}
		log.Println("failed to search alcohol:", err)
		return nil, fmt.Errorf("failed to search alcohol: %w", err)
	}

	log.Printf("found alcohol item: %s", item.Name)

	insertQuery := `
    INSERT INTO alcohol_collection (user_id, alcohol_id)
    VALUES ($1, $2)
    ON CONFLICT (user_id, alcohol_id) DO NOTHING
    RETURNING id, acquired_at
    `

	var collectionID string
	var acquiredAt time.Time
	err = s.db.QueryRow(ctx, insertQuery, userID, item.ID).Scan(&collectionID, &acquiredAt)

	isNewlyAdded := true
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("alcohol item %s already in user's collection", item.Name)
			isNewlyAdded = false
		} else {
			log.Println("failed to add to collection:", err)
			return nil, fmt.Errorf("failed to add to collection: %w", err)
		}
	} else {
		log.Printf("added alcohol item %s to user's collection", item.Name)
	}

	result := map[string]interface{}{
		"item":         &item,
		"isNewlyAdded": isNewlyAdded,
	}

	return result, nil
}

func (s *UserService) GetUserAlcoholCollection(ctx context.Context, clerkID string) (collection.AlcoholCollectionByType, error) {
	log.Println("fetching user alcohol collection")

	var userID string
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		log.Println("failed to get user ID:", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	query := `
    SELECT 
        d.id,
        d.name,
        d.type,
        d.image_url,
        d.rarity,
        d.abv,
        c.acquired_at
    FROM alcohol_collection c
    INNER JOIN db_alcohol_collection_data d ON c.alcohol_id = d.id
    WHERE c.user_id = $1
    ORDER BY c.acquired_at DESC
    `

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get user collection:", err)
		return nil, fmt.Errorf("failed to get user collection: %w", err)
	}
	defer rows.Close()

	collectionItemTypes := collection.AlcoholCollectionByType{
		"beer":    []collection.AlcoholItem{},
		"whiskey": []collection.AlcoholItem{},
		"wine":    []collection.AlcoholItem{},
		"vodka":   []collection.AlcoholItem{},
		"gin":     []collection.AlcoholItem{},
		"liqueur": []collection.AlcoholItem{},
		"rum":     []collection.AlcoholItem{},
		"tequila": []collection.AlcoholItem{},
		"rakiya":  []collection.AlcoholItem{},
	}

	for rows.Next() {
		var item collection.AlcoholItem
		var acquiredAt time.Time

		err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Type,
			&item.ImageUrl,
			&item.Rarity,
			&item.Abv,
			&acquiredAt,
		)

		if err != nil {
			log.Println("failed to scan alcohol item:", err)
			return nil, fmt.Errorf("failed to scan alcohol item: %w", err)
		}

		alcoholType := strings.ToLower(item.Type)

		if _, exists := collectionItemTypes[alcoholType]; exists {
			collectionItemTypes[alcoholType] = append(collectionItemTypes[alcoholType], item)
		} else {
			log.Printf("unknown alcohol type: %s for item: %s", item.Type, item.Name)
		}
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating collection:", err)
		return nil, fmt.Errorf("error iterating collection: %w", err)
	}

	log.Printf("fetched user collection: %d items", getTotalCount(collectionItemTypes))
	return collectionItemTypes, nil
}

func (s *UserService) RemoveAlcoholCollectionItem(ctx context.Context, clerkID string, itemIdForRemoval string) (bool, error) {
	log.Println("removing item id from user collection")

	var userID string
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		log.Println("failed to get user ID:", err)
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	query := `
   DELETE FROM alcohol_collection ac 
	WHERE ac.user_id = $1 AND ac.alcohol_id = $2
    `

	rows, err := s.db.Query(ctx, query, userID, itemIdForRemoval)
	if err != nil {
		log.Println("failed to get user collection:", err)
		return false, fmt.Errorf("failed to get user collection: %w", err)
	}
	defer rows.Close()

	return true, nil
}

func (s *UserService) GetAlcoholismChart(ctx context.Context, clerkID string, period string) ([]byte, error) {
	var userID string
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}

	query := `SELECT get_alcoholism_chart_data($1, $2)`

	var jsonResult []byte

	err = s.db.QueryRow(ctx, query, userID, period).Scan(&jsonResult)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chart data: %w", err)
	}

	if len(jsonResult) == 0 {
		return []byte("[]"), nil
	}

	return jsonResult, nil
}

func (s *UserService) GetUserInventory(ctx context.Context, clerkID string) (map[string][]*store.InventoryItem, error) {
	var userID uuid.UUID
	userQuery := `SELECT id FROM users WHERE clerk_id = $1`
	err := s.db.QueryRow(ctx, userQuery, clerkID).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	query := `
		SELECT
			i.id,
			i.user_id,
			i.item_id,
			i.item_type,
			i.quantity,
			i.is_equipped,
			i.acquired_at,
			i.expires_at
		FROM user_inventory i
		WHERE i.user_id = $1
		ORDER BY i.acquired_at DESC
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory: %w", err)
	}
	defer rows.Close()

	var inventory = make(map[string][]*store.InventoryItem)
	for rows.Next() {
		var item store.InventoryItem
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.ItemID,
			&item.ItemType,
			&item.Quantity,
			&item.IsEquipped,
			&item.AcquiredAt,
			&item.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
		}
		inventory[item.ItemType] = append(inventory[item.ItemType], &item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory rows: %w", err)
	}

	return inventory, nil
}

type StorySegment struct {
	ID            uuid.UUID `json:"id"`
	VideoUrl      string    `json:"video_url"`
	VideoWidth    uint      `json:"video_width"`
	VideoHeight   uint      `json:"video_height"`
	VideoDuration uint      `json:"video_duration"`
	RelateCount   int       `json:"relate_count"`
	HasRelated    bool      `json:"has_related"`
	IsSeen        bool      `json:"is_seen"`
	CreatedAt     time.Time `json:"created_at"`
}

type UserStories struct {
	UserID       uuid.UUID      `json:"user_id"`
	Username     string         `json:"username"`
	UserImageUrl string         `json:"user_image_url"`
	AllSeen      bool           `json:"all_seen"`
	Items        []StorySegment `json:"items"`
}

func (s *UserService) GetStories(ctx context.Context, clerkID string) ([]UserStories, error) {
	query := `
		WITH flat_stories AS (
			SELECT 
				s.id, 
				s.user_id, 
				author.username,
				author.image_url, 
				s.video_url, 
				s.video_width, 
				s.video_height, 
				s.video_duration, 
				s.created_at,
				(SELECT COUNT(*) FROM relates WHERE story_id = s.id) as relate_count,
				EXISTS(SELECT 1 FROM relates r WHERE r.story_id = s.id AND r.user_id = viewer.id) as has_related,
				(viewer.id = ANY(COALESCE(s.seen_by, '{}'))) as is_seen
			FROM stories s
			JOIN users author ON author.id = s.user_id 
			CROSS JOIN users viewer                   
			WHERE viewer.clerk_id = $1
			AND s.expires_at > NOW()
			AND s.visibility = 'friends'
			AND (
				s.user_id = viewer.id 
				OR s.user_id IN (SELECT friend_id FROM friendships WHERE user_id = viewer.id AND status = 'accepted')
			)
		)
		SELECT 
			user_id,
			username,
			image_url,
			BOOL_AND(is_seen) as all_seen, -- True only if ALL stories are seen
			json_agg(
				json_build_object(
					'id', id,
					'video_url', video_url,
					'video_width', video_width,
					'video_height', video_height,
					'video_duration', video_duration,
					'relate_count', relate_count,
					'has_related', has_related,
					'is_seen', is_seen,
					'created_at', created_at
				) ORDER BY created_at ASC
			) as items
		FROM flat_stories
		GROUP BY user_id, username, image_url
		ORDER BY MAX(created_at) DESC`

	rows, err := s.db.Query(ctx, query, clerkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UserStories
	for rows.Next() {
		var u UserStories
		err := rows.Scan(&u.UserID, &u.Username, &u.UserImageUrl, &u.AllSeen, &u.Items)
		if err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, nil
}

func (s *UserService) AddStory(ctx context.Context, clerkID string, videoUrl string, videoWidth float64, videoHeight float64, videoDuration float64, taggedBuddiesIds []string) (bool, error) {
	userID, username, err := s.getInternalID(ctx, clerkID)
	if err != nil {
		return false, err
	}

	var storyId uuid.UUID

	query := `
		INSERT INTO stories (user_id, video_url, video_width, video_height, video_duration)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	err = s.db.QueryRow(ctx, query, userID, videoUrl, videoWidth, videoHeight, videoDuration).Scan(&storyId)
	if err != nil {
		return false, err
	}

	//!TEST THIS LOGIC
	go utils.FriendPostedStory(s.db, s.notifService, userID, username, videoUrl, storyId)
	return true, nil
}

func (s *UserService) RelateStory(ctx context.Context, clerkID, storyID, action string) (bool, error) {
	userID, _, err := s.getInternalID(ctx, clerkID)
	if err != nil {
		return false, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	// Attempt to delete the specific reaction type for this user
	cmd, err := tx.Exec(ctx, `
		DELETE FROM relates 
		WHERE story_id = $1 AND user_id = $2 AND relate_type = $3`,
		storyID, userID, action,
	)
	if err != nil {
		return false, err
	}

	// If nothing was deleted, it means the user hasn't reacted with this type yet -> Insert it
	if cmd.RowsAffected() == 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO relates (story_id, user_id, relate_type) 
			VALUES ($1, $2, $3)`,
			storyID, userID, action,
		)
		if err != nil {
			return false, err
		}
	}

	err = tx.Commit(ctx)
	return true, err
}

// MarkStoryAsSeen adds the userID to the seen_by array in the stories table
func (s *UserService) MarkStoryAsSeen(ctx context.Context, clerkID, storyID string) (bool, error) {
	userID, _, err := s.getInternalID(ctx, clerkID)
	if err != nil {
		return false, err
	}

	// We use array_append combined with a check to ensure we don't add the same user twice.
	// The "NOT ($1 = ANY(seen_by))" check prevents duplicates in the array.
	_, err = s.db.Exec(ctx, `
		UPDATE stories 
		SET seen_by = array_append(seen_by, $1) 
		WHERE id = $2 
		AND NOT ($1 = ANY(COALESCE(seen_by, '{}')))`,
		userID, storyID,
	)

	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *UserService) GetAllUserStories(ctx context.Context, clerkID string) ([]story.Story, error) {
	query := `
		SELECT 
			s.id, s.user_id, s.video_url, s.video_width, s.video_height, s.video_duration, s.created_at,
			(SELECT COUNT(*) FROM relates WHERE story_id = s.id) as relate_count
		FROM stories s
		JOIN users u ON u.id = s.user_id
		WHERE u.clerk_id = $1
		ORDER BY s.created_at DESC`

	rows, err := s.db.Query(ctx, query, clerkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []story.Story
	for rows.Next() {
		var st story.Story
		err := rows.Scan(&st.ID, &st.UserID, &st.VideoUrl, &st.VideoWidth, &st.VideoHeight, &st.VideoDuration, &st.CreatedAt, &st.RelateCount)
		if err != nil {
			return nil, err
		}
		stories = append(stories, st)
	}
	return stories, nil
}

func (s *UserService) GetSubscriptionDetails(ctx context.Context, clerkID string) (*subscription.Subscription, error) {
	query := `
		SELECT 
			s.id, s.user_id, s.stripe_customer_id, s.stripe_subscription_id, 
			s.stripe_price_id, s.status, s.current_period_end, s.created_at, s.updated_at
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		WHERE u.clerk_id = $1
		-- Optional: Only fetch active ones? 
		-- usually you want the latest one regardless of status to show "Cancelled" logic
		ORDER BY s.created_at DESC 
		LIMIT 1`

	row := s.db.QueryRow(ctx, query, clerkID)

	var sub subscription.Subscription
	err := row.Scan(
		&sub.ID, &sub.UserID, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.StripePriceID, &sub.Status, &sub.CurrentPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User has no subscription record. Return nil, no error.
			return nil, nil
		}
		return nil, err
	}

	return &sub, nil
}

// Subscribe generates a Stripe Checkout Session URL
func (s *UserService) Subscribe(ctx context.Context, clerkID string, priceID string) (string, error) {
	// 1. Get the user's email and internal ID from DB
	var email string
	var userID string
	err := s.db.QueryRow(ctx, "SELECT id, email FROM users WHERE clerk_id = $1", clerkID).Scan(&userID, &email)
	if err != nil {
		return "", errors.New("user not found")
	}

	// 2. Initialize Stripe Key (Make sure STRIPE_SECRET_KEY is in your .env)
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	// 3. Create Checkout Session Params
	params := &stripe.CheckoutSessionParams{
		CustomerEmail: stripe.String(email), // Pre-fill user email
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		// "subscription" mode is required for recurring payments
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID), // The ID from frontend (monthly or yearly)
				Quantity: stripe.Int64(1),
			},
		},
		// Metadata helps us identify the user in the Webhook later
		Metadata: map[string]string{
			"user_id":  userID,
			"clerk_id": clerkID,
		},
		SuccessURL: stripe.String("http://localhost:3000/settings?payment=success"), // Update with your frontend URL
		CancelURL:  stripe.String("http://localhost:3000/settings?payment=canceled"),
	}

	// 4. Call Stripe API
	sess, err := session.New(params)
	if err != nil {
		return "", err
	}

	// 5. Return the URL
	return sess.URL, nil
}

func (s *UserService) DeleteStory(ctx context.Context, clerkID string, storyID string) (bool, error) {
	// 1. Get the internal User UUID
	// Since getInternalID returns a uuid.UUID, this part is safe.
	userID, _, err := s.getInternalID(ctx, clerkID)
	if err != nil {
		return false, err
	}

	// 2. FIX: Parse the storyID string into a UUID
	// This catches the empty string case ("") before it hits the database.
	storyUUID, err := uuid.Parse(storyID)
	if err != nil {
		return false, fmt.Errorf("invalid story ID: %w", err)
	}

	// 3. Execute Query using both UUIDs
	query := `DELETE FROM stories WHERE id = $1 AND user_id = $2`

	// Pass storyUUID (type uuid.UUID) instead of storyID (type string)
	cmd, err := s.db.Exec(ctx, query, storyUUID, userID)
	if err != nil {
		return false, err
	}

	return cmd.RowsAffected() > 0, nil
}

// TODO: Creae theese
func (s *UserService) EquipItem(ctx context.Context, clerkID string, itemIdForRemoval string) (bool, error) {
	return true, nil
}
func (s *UserService) FetchStripeSubscription(subID string) (*stripe.Subscription, error) {
	return stripeClient.Get(subID, nil)
}

func (s *UserService) UpsertSubscription(ctx context.Context, sub *subscription.Subscription) error {
	query := `
		INSERT INTO subscriptions (
			user_id, 
			stripe_customer_id, 
			stripe_subscription_id, 
			stripe_price_id, 
			status, 
			current_period_end,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (stripe_subscription_id) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			current_period_end = EXCLUDED.current_period_end,
			stripe_price_id = EXCLUDED.stripe_price_id,
			updated_at = NOW();
	`
	_, err := s.db.Exec(ctx, query,
		sub.UserID,
		sub.StripeCustomerID,
		sub.StripeSubscriptionID,
		sub.StripePriceID,
		sub.Status,
		sub.CurrentPeriodEnd,
	)
	return err
}

func (s *UserService) UpdateSubscriptionStatus(ctx context.Context, sub *subscription.Subscription) error {
	query := `
        UPDATE subscriptions 
        SET 
            status = $1, 
            current_period_end = $2, 
            stripe_price_id = $3,
            updated_at = NOW()
        WHERE stripe_subscription_id = $4
    `
	result, err := s.db.Exec(ctx, query,
		sub.Status,
		sub.CurrentPeriodEnd,
		sub.StripePriceID,
		sub.StripeSubscriptionID,
	)
	if err != nil {
		return err
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("subscription %s not found locally", sub.StripeSubscriptionID)
	}
	return nil
}

func getTotalCount(collection collection.AlcoholCollectionByType) int {
	total := 0
	for _, items := range collection {
		total += len(items)
	}
	return total
}

func (s *UserService) getInternalID(ctx context.Context, clerkID string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var username string

	err := s.db.QueryRow(ctx, `SELECT id, username FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID, &username)
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, "", fmt.Errorf("user not found")
		}
		return uuid.Nil, "", err
	}
	return userID, username, nil
}
