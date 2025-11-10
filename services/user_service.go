package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"outDrinkMeAPI/internal/achievement"
	"outDrinkMeAPI/internal/calendar"
	"outDrinkMeAPI/internal/leaderboard"
	"outDrinkMeAPI/internal/stats"
	"outDrinkMeAPI/internal/user"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	db *pgxpool.Pool
}

func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{db: db}
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
	SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at, gems, xp, all_days_drinking_count
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
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *UserService) FriendDiscoveryDisplayProfile(ctx context.Context, clerkID string, FriendDiscoveryId string) (*user.FriendDiscoveryDisplayProfileResponse, error) {
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
    SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at, gems, xp, all_days_drinking_count
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("FriendDiscoveryDisplayProfile: User not found for UUID: %s", friendDiscoveryUUID)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("FriendDiscoveryDisplayProfile: Failed to get user: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Use the clerk_id from the retrieved user data
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

	// Check if they are friends
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
		// Don't fail the whole request, just set isFriend to false
		isFriend = false
	}

	log.Println("Friend Discovery User Data:", friendDiscoveryUserData)
	log.Println("Friend Discovery Stats:", friendDiscoveryStats)
	log.Println("Friend Discovery Achievements:", friendDiscoveryAchievements)
	log.Println("Is Friend:", isFriend)

	response := &user.FriendDiscoveryDisplayProfileResponse{
		User:         friendDiscoveryUserData,
		Stats:        friendDiscoveryStats,
		Achievements: friendDiscoveryAchievements,
		IsFriend:     isFriend,
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
	// Get current user ID
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		log.Printf("AddFriend: Failed to find user with clerk_id %s: %v", clerkID, err)
		return fmt.Errorf("user not found")
	}

	// Get friend user ID
	var friendID uuid.UUID
	err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, friendClerkID).Scan(&friendID)
	if err != nil {
		log.Printf("AddFriend: Failed to find friend with clerk_id %s: %v", friendClerkID, err)
		return fmt.Errorf("friend user not found")
	}

	// Prevent adding yourself as a friend
	if userID == friendID {
		log.Printf("AddFriend: User %s attempted to add themselves", clerkID)
		return fmt.Errorf("cannot add yourself as a friend")
	}

	// Check if friendship already exists (in either direction)
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

	// Insert the friendship (without updated_at)
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
	// Get current user ID
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		log.Printf("RemoveFriend: Failed to find user with clerk_id %s: %v", clerkID, err)
		return fmt.Errorf("user not found")
	}

	// Get friend user ID
	var friendID uuid.UUID
	err = s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, friendClerkID).Scan(&friendID)
	if err != nil {
		log.Printf("RemoveFriend: Failed to find friend with clerk_id %s: %v", friendClerkID, err)
		return fmt.Errorf("friend user not found")
	}

	// Delete the friendship (check both directions)
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

func (s *UserService) GetFriendsLeaderboard(ctx context.Context, clerkID string) (*leaderboard.Leaderboard, error) {
	// Get user ID from clerk_id
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		u.id as user_id,
		u.username,
		u.image_url,
		COALESCE(ws.days_drank, 0) as days_this_week,
		RANK() OVER (ORDER BY COALESCE(ws.days_drank, 0) DESC) as rank,
		COALESCE(s.current_streak, 0) as current_streak
	FROM users u
	INNER JOIN friendships f 
		ON (f.friend_id = u.id AND f.user_id = $1 AND f.status = 'accepted')
	LEFT JOIN weekly_stats ws 
		ON u.id = ws.user_id 
		AND ws.week_start = DATE_TRUNC('week', CURRENT_DATE)::DATE
	LEFT JOIN streaks s 
		ON u.id = s.user_id
	ORDER BY days_this_week DESC, current_streak DESC
	LIMIT 50
`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []*leaderboard.LeaderboardEntry
	var userPosition *leaderboard.LeaderboardEntry

	for rows.Next() {
		entry := &leaderboard.LeaderboardEntry{}
		err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.ImageURL,
			&entry.DaysThisWeek,
			&entry.Rank,
			&entry.CurrentStreak,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		entries = append(entries, entry)

		if entry.UserID == userID {
			userPosition = entry
		}
	}

	return &leaderboard.Leaderboard{
		Entries:      entries,
		UserPosition: userPosition,
		TotalUsers:   len(entries),
	}, nil
}

func (s *UserService) GetGlobalLeaderboard(ctx context.Context, clerkID string) (*leaderboard.Leaderboard, error) {
	// Get user ID from clerk_id
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		u.id AS user_id,
		u.username,
		u.image_url,
		COALESCE(ws.days_drank, 0) AS days_this_week,
		RANK() OVER (ORDER BY COALESCE(ws.days_drank, 0) DESC) AS rank,
		COALESCE(s.current_streak, 0) AS current_streak
	FROM users u
	LEFT JOIN weekly_stats ws 
		ON u.id = ws.user_id 
		AND ws.week_start = DATE_TRUNC('week', CURRENT_DATE)::DATE
	LEFT JOIN streaks s 
		ON u.id = s.user_id
	ORDER BY days_this_week DESC, current_streak DESC
	LIMIT 50
`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []*leaderboard.LeaderboardEntry
	var userPosition *leaderboard.LeaderboardEntry

	for rows.Next() {
		entry := &leaderboard.LeaderboardEntry{}
		err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.ImageURL,
			&entry.DaysThisWeek,
			&entry.Rank,
			&entry.CurrentStreak,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		entries = append(entries, entry)

		if entry.UserID == userID {
			userPosition = entry
		}
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


func (s *UserService) AddDrinking(ctx context.Context, clerkID string, drankToday bool, imageUrl *string, locationText *string, clerkIDs []string, date time.Time) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
        INSERT INTO daily_drinking (user_id, date, drank_today, logged_at, image_url, location_text, mentioned_buddies)
        VALUES ($1, $2, $3, NOW(), $4, $5, $6)
        ON CONFLICT (user_id, date) 
        DO UPDATE SET 
            drank_today = $3, 
            logged_at = NOW(), 
            image_url = $4, 
            location_text = $5, 
            mentioned_buddies = $6
    `

	_, err = s.db.Exec(ctx, query, userID, date, drankToday, imageUrl, locationText, clerkIDs)
	if err != nil {
		return fmt.Errorf("failed to log drinking: %w", err)
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
			// No entry for that date
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

	// Return empty slice instead of nil if no users found
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

func (s *UserService) GetCalendar(ctx context.Context, clerkID string, year int, month int) (*calendar.CalendarResponse, error) {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, -1)

	query := `
	SELECT date, drank_today
	FROM daily_drinking
	WHERE user_id = $1
		AND date >= $2
		AND date <= $3
	ORDER BY date
	`

	rows, err := s.db.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar: %w", err)
	}
	defer rows.Close()

	dayMap := make(map[string]bool)
	for rows.Next() {
		var date time.Time
		var drank bool
		if err := rows.Scan(&date, &drank); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		dayMap[date.Format("2006-01-02")] = drank
	}

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

	// Calculate alcoholism coefficient: currentStreak^2 + totalDays*0.3 + achievements*5
	alcoholismScore := float64(stats.CurrentStreak*stats.CurrentStreak)*0.3 +
		(float64(stats.TotalDaysDrank) * 0.05) +
		(float64(stats.AchievementsCount) * 1)

	stats.AlcoholismCoefficient = alcoholismScore

	// Calculate rank based on AlcoholismCoefficient
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

type DailyDrinkingPost struct {
	ID               string
	UserID           string
	UserImageURL     *string
	Date             time.Time
	DrankToday       bool
	LoggedAt         time.Time
	ImageURL         *string
	LocationText     *string
	MentionedBuddies []user.User
	SourceType       string
}

func (s *UserService) GetYourMix(ctx context.Context, clerkID string) ([]DailyDrinkingPost, error) {
	log.Println("getting feed")

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	WITH friend_posts AS (
		-- Get posts from friends (up to 30 posts = 60% of 50)
		SELECT 
			dd.id,
			dd.user_id,
			u.image_url AS user_image_url,
			dd.date,
			dd.drank_today,
			dd.logged_at,
			dd.image_url AS post_image_url,
			dd.location_text,
			dd.mentioned_buddies,
			'friend' AS source_type
		FROM daily_drinking dd
		JOIN users u ON u.id = dd.user_id
		WHERE dd.user_id != $1
			AND dd.logged_at >= NOW() - INTERVAL '5 days'
			AND dd.image_url IS NOT NULL
			AND dd.image_url != ''
			AND dd.user_id IN (
				-- Get all friends (bidirectional)
				SELECT friend_id FROM friendships 
				WHERE user_id = $1 AND status = 'accepted'
				UNION
				SELECT user_id FROM friendships 
				WHERE friend_id = $1 AND status = 'accepted'
			)
		ORDER BY dd.logged_at DESC
		LIMIT 30
	),
	friend_count AS (
		-- Count how many friend posts we got
		SELECT COUNT(*) as cnt FROM friend_posts
	),
	other_posts AS (
		-- Get posts from non-friends
		-- Calculate limit: 50 - friend_count, with minimum of 20 (40% of 50)
		SELECT 
			dd.id,
			dd.user_id,
			u.image_url AS user_image_url,
			dd.date,
			dd.drank_today,
			dd.logged_at,
			dd.image_url AS post_image_url,
			dd.location_text,
			dd.mentioned_buddies,
			'other' AS source_type
		FROM daily_drinking dd
		JOIN users u ON u.id = dd.user_id
		WHERE dd.user_id != $1
			AND dd.logged_at >= NOW() - INTERVAL '5 days'
			AND dd.image_url IS NOT NULL
			AND dd.image_url != ''
			AND dd.user_id NOT IN (
				-- Exclude friends (bidirectional)
				SELECT friend_id FROM friendships 
				WHERE user_id = $1 AND status = 'accepted'
				UNION
				SELECT user_id FROM friendships 
				WHERE friend_id = $1 AND status = 'accepted'
			)
		ORDER BY dd.logged_at DESC
		LIMIT GREATEST(20, 50 - (SELECT cnt FROM friend_count))
	)
	-- Combine and return final feed
	SELECT 
		id,
		user_id,
		user_image_url,
		date,
		drank_today,
		logged_at,
		post_image_url,
		location_text,
		mentioned_buddies,
		source_type
	FROM (
		SELECT * FROM friend_posts
		UNION ALL
		SELECT * FROM other_posts
	) AS combined_feed
	ORDER BY logged_at DESC
	LIMIT 50
	`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		log.Println("failed to get feed")
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}
	defer rows.Close()

	var posts []DailyDrinkingPost
	for rows.Next() {
		var post DailyDrinkingPost
		var mentionedBuddyIDs []string // Scan as string array

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
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

		// Fetch the full user objects for mentioned buddies
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

func (s *UserService) GetMixTimeline(ctx context.Context, clerkID string) ([]DailyDrinkingPost, error) {
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

	var posts []DailyDrinkingPost
	for rows.Next() {
		var post DailyDrinkingPost
		var mentionedBuddyIDs []string // Scan as string array

		err := rows.Scan(
			&post.ID,
			&post.UserID,
			&post.UserImageURL,
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

		// Fetch the full user objects for mentioned buddies
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

type DrunkThought struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	UserImageURL string    `json:"user_image_url"`
	Thought      string    `json:"thought"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *UserService) GetDrunkFriendThoughts(ctx context.Context, clerkID string) ([]DrunkThought, error) {
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

	var thoughts []DrunkThought
	for rows.Next() {
		var thought DrunkThought
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

func (s *UserService) GetAlcoholCollection(ctx context.Context, clerkID string) ([]DrunkThought, error) {
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

	var thoughts []DrunkThought
	for rows.Next() {
		var thought DrunkThought
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

type AlcoholItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	ImageUrl *string `json:"image_url"`
	Rarity   string  `json:"rarity"`
	Abv      float32 `json:"abv"`
}

func (s *UserService) SearchDbAlcohol(ctx context.Context, clerkID string, queryAlcoholName string) (map[string]interface{}, error) {
	log.Println("searching alcohol collection")

	// First, get the user's UUID from clerk_id
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

	var item AlcoholItem
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

	// Add to user's collection (insert or ignore if already exists)
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
			// Item already exists in collection
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
		"item":       &item,
		"isNewlyAdded": isNewlyAdded,
	}

	return result, nil
}
type AlcoholCollectionByType map[string][]AlcoholItem

func (s *UserService) GetUserAlcoholCollection(ctx context.Context, clerkID string) (AlcoholCollectionByType, error) {
	log.Println("fetching user alcohol collection")

	// First, get the user's UUID from clerk_id
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

	// Initialize the map with all alcohol types
	collection := AlcoholCollectionByType{
		"beer":    []AlcoholItem{},
		"whiskey": []AlcoholItem{},
		"wine":    []AlcoholItem{},
		"vodka":   []AlcoholItem{},
		"gin":     []AlcoholItem{},
		"liqueur": []AlcoholItem{},
		"rum":     []AlcoholItem{},
		"tequila": []AlcoholItem{},
		"rakiya":  []AlcoholItem{},
	}

	for rows.Next() {
		var item AlcoholItem
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

		// Normalize the type to lowercase for consistent map keys
		alcoholType := strings.ToLower(item.Type)

		// Add to the appropriate category
		if _, exists := collection[alcoholType]; exists {
			collection[alcoholType] = append(collection[alcoholType], item)
		} else {
			// If type doesn't match any category, you can either skip it or add to "other"
			log.Printf("unknown alcohol type: %s for item: %s", item.Type, item.Name)
		}
	}

	if err = rows.Err(); err != nil {
		log.Println("error iterating collection:", err)
		return nil, fmt.Errorf("error iterating collection: %w", err)
	}

	log.Printf("fetched user collection: %d items", getTotalCount(collection))
	return collection, nil
}

// Helper function to count total items
func getTotalCount(collection AlcoholCollectionByType) int {
	total := 0
	for _, items := range collection {
		total += len(items)
	}
	return total
}
