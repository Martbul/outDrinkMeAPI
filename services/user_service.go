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

// func (s *UserService) GetUserFullProfile(ctx context.Context, requestingClerkID string, targetUserID string) (*UserProfileResponse, error) {
// 	// Verify requesting user exists
// 	var requestingUserID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, requestingClerkID).Scan(&requestingUserID)
// 	if err != nil {
// 		log.Printf("GetUserFullProfile: Failed to find requesting user: %v", err)
// 		return nil, fmt.Errorf("user not authenticated")
// 	}

// 	// Parse target user UUID
// 	targetUUID, err := uuid.Parse(targetUserID)
// 	if err != nil {
// 		log.Printf("GetUserFullProfile: Invalid target user ID %s: %v", targetUserID, err)
// 		return nil, fmt.Errorf("invalid user id")
// 	}

// 	// Get target user's profile
// 	userQuery := `
// 		SELECT id, clerk_id, email, username, first_name, last_name, image_url,
// 		       email_verified, created_at, updated_at, gems, xp, all_days_drinking_count
// 		FROM users
// 		WHERE id = $1
// 	`

// 	targetUser := &user.User{}
// 	err = s.db.QueryRow(ctx, userQuery, targetUUID).Scan(
// 		&targetUser.ID,
// 		&targetUser.ClerkID,
// 		&targetUser.Email,
// 		&targetUser.Username,
// 		&targetUser.FirstName,
// 		&targetUser.LastName,
// 		&targetUser.ImageURL,
// 		&targetUser.EmailVerified,
// 		&targetUser.CreatedAt,
// 		&targetUser.UpdatedAt,
// 		&targetUser.Gems,
// 		&targetUser.XP,
// 		&targetUser.AllDaysDrinkingCount,
// 	)

// 	if err != nil {
// 		if errors.Is(err, pgx.ErrNoRows) {
// 			return nil, fmt.Errorf("user not found")
// 		}
// 		return nil, fmt.Errorf("failed to get user: %w", err)
// 	}

// 	// Check if they are friends
// 	var isFriend bool
// 	friendCheckQuery := `
// 		SELECT EXISTS(
// 			SELECT 1 FROM friendships
// 			WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
// 			AND status = 'accepted'
// 		)
// 	`
// 	s.db.QueryRow(ctx, friendCheckQuery, requestingUserID, targetUUID).Scan(&isFriend)

// 	// Get today's status
// 	var todayStatus bool
// 	todayQuery := `
// 		SELECT EXISTS(
// 			SELECT 1 FROM drinking_calendar
// 			WHERE user_id = $1 AND date = CURRENT_DATE AND drank = true
// 		)
// 	`
// 	s.db.QueryRow(ctx, todayQuery, targetUUID).Scan(&todayStatus)

// 	// Get days this week
// 	var daysThisWeek int
// 	weekQuery := `
// 		SELECT COALESCE(COUNT(*), 0)
// 		FROM drinking_calendar
// 		WHERE user_id = $1
// 		AND date >= date_trunc('week', CURRENT_DATE)
// 		AND date < date_trunc('week', CURRENT_DATE) + INTERVAL '7 days'
// 		AND drank = true
// 	`
// 	s.db.QueryRow(ctx, weekQuery, targetUUID).Scan(&daysThisWeek)

// 	// Get days this month
// 	var daysThisMonth int
// 	monthQuery := `
// 		SELECT COALESCE(COUNT(*), 0)
// 		FROM drinking_calendar
// 		WHERE user_id = $1
// 		AND date >= date_trunc('month', CURRENT_DATE)
// 		AND date < date_trunc('month', CURRENT_DATE) + INTERVAL '1 month'
// 		AND drank = true
// 	`
// 	s.db.QueryRow(ctx, monthQuery, targetUUID).Scan(&daysThisMonth)

// 	// Get days this year
// 	var daysThisYear int
// 	yearQuery := `
// 		SELECT COALESCE(COUNT(*), 0)
// 		FROM drinking_calendar
// 		WHERE user_id = $1
// 		AND date >= date_trunc('year', CURRENT_DATE)
// 		AND date < date_trunc('year', CURRENT_DATE) + INTERVAL '1 year'
// 		AND drank = true
// 	`
// 	s.db.QueryRow(ctx, yearQuery, targetUUID).Scan(&daysThisYear)

// 	// Get total days drank
// 	var totalDaysDrank int
// 	s.db.QueryRow(ctx, `SELECT COALESCE(all_days_drinking_count, 0) FROM users WHERE id = $1`, targetUUID).Scan(&totalDaysDrank)

// 	// Calculate current streak
// 	currentStreak := s.calculateStreak(ctx, targetUUID, true)

// 	// Calculate longest streak
// 	longestStreak := s.calculateLongestStreak(ctx, targetUUID)

// 	// Get total weeks won
// 	var totalWeeksWon int
// 	weeksWonQuery := `
// 		SELECT COALESCE(SUM(win_count), 0)
// 		FROM weekly_stats
// 		WHERE user_id = $1
// 	`
// 	s.db.QueryRow(ctx, weeksWonQuery, targetUUID).Scan(&totalWeeksWon)

// 	// Get achievements count
// 	var achievementsCount int
// 	achievementsQuery := `
// 		SELECT COALESCE(COUNT(*), 0)
// 		FROM user_achievements
// 		WHERE user_id = $1
// 	`
// 	s.db.QueryRow(ctx, achievementsQuery, targetUUID).Scan(&achievementsCount)

// 	// Get friends count
// 	var friendsCount int
// 	friendsQuery := `
// 		SELECT COALESCE(COUNT(*), 0)
// 		FROM friendships
// 		WHERE (user_id = $1 OR friend_id = $1) AND status = 'accepted'
// 	`
// 	s.db.QueryRow(ctx, friendsQuery, targetUUID).Scan(&friendsCount)

// 	// Calculate alcoholism coefficient
// 	var alcoholismCoefficient float64
// 	if totalDaysDrank > 0 {
// 		var accountAgeDays int
// 		ageQuery := `
// 			SELECT EXTRACT(DAY FROM CURRENT_DATE - created_at::date)
// 			FROM users
// 			WHERE id = $1
// 		`
// 		s.db.QueryRow(ctx, ageQuery, targetUUID).Scan(&accountAgeDays)

// 		if accountAgeDays > 0 {
// 			alcoholismCoefficient = float64(totalDaysDrank) / float64(accountAgeDays) * 100
// 		}
// 	}

// 	// Get rank
// 	var rank int
// 	rankQuery := `
// 		WITH ranked_users AS (
// 			SELECT id, ROW_NUMBER() OVER (ORDER BY all_days_drinking_count DESC, xp DESC) as rank
// 			FROM users
// 		)
// 		SELECT rank FROM ranked_users WHERE id = $1
// 	`
// 	s.db.QueryRow(ctx, rankQuery, targetUUID).Scan(&rank)

// 	// Build UserStats
// 	userStats := &stats.UserStats{
// 		TodayStatus:           todayStatus,
// 		DaysThisWeek:          daysThisWeek,
// 		DaysThisMonth:         daysThisMonth,
// 		DaysThisYear:          daysThisYear,
// 		TotalDaysDrank:        totalDaysDrank,
// 		CurrentStreak:         currentStreak,
// 		LongestStreak:         longestStreak,
// 		TotalWeeksWon:         totalWeeksWon,
// 		AchievementsCount:     achievementsCount,
// 		FriendsCount:          friendsCount,
// 		AlcoholismCoefficient: alcoholismCoefficient,
// 		Rank:                  rank,
// 	}

// 	// Get current week stats
// 	weeklyStats := &weekly_stats.WeeklyStats{}
// 	weeklyStatsQuery := `
// 		SELECT id, user_id, week_start, week_end, days_drank, total_days, win_count, created_at, updated_at
// 		FROM weekly_stats
// 		WHERE user_id = $1
// 		AND week_start = date_trunc('week', CURRENT_DATE)
// 		LIMIT 1
// 	`
// 	err = s.db.QueryRow(ctx, weeklyStatsQuery, targetUUID).Scan(
// 		&weeklyStats.ID,
// 		&weeklyStats.UserID,
// 		&weeklyStats.WeekStart,
// 		&weeklyStats.WeekEnd,
// 		&weeklyStats.DaysDrank,
// 		&weeklyStats.TotalDays,
// 		&weeklyStats.WinCount,
// 		&weeklyStats.CreatedAt,
// 		&weeklyStats.UpdatedAt,
// 	)
// 	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
// 		log.Printf("GetUserFullProfile: Failed to get weekly stats: %v", err)
// 		// Continue without weekly stats
// 		weeklyStats = nil
// 	}

// 	// Get achievements
// 	achievementsListQuery := `
// 		SELECT
// 			a.id,
// 			a.name,
// 			a.description,
// 			a.icon,
// 			a.criteria_type,
// 			a.criteria_value,
// 			a.created_at,
// 			CASE WHEN ua.id IS NOT NULL THEN true ELSE false END as unlocked,
// 			ua.unlocked_at
// 		FROM achievements a
// 		LEFT JOIN user_achievements ua ON a.id = ua.achievement_id AND ua.user_id = $1
// 		ORDER BY a.criteria_value ASC, a.created_at ASC
// 	`

// 	rows, err := s.db.Query(ctx, achievementsListQuery, targetUUID)
// 	if err != nil {
// 		log.Printf("GetUserFullProfile: Failed to fetch achievements: %v", err)
// 		return nil, fmt.Errorf("failed to fetch achievements")
// 	}
// 	defer rows.Close()

// 	var achievementsList []achievement.AchievementWithStatus
// 	for rows.Next() {
// 		var a achievement.AchievementWithStatus
// 		err := rows.Scan(
// 			&a.ID,
// 			&a.Name,
// 			&a.Description,
// 			&a.Icon,
// 			&a.CriteriaType,
// 			&a.CriteriaValue,
// 			&a.CreatedAt,
// 			&a.Unlocked,
// 			&a.UnlockedAt,
// 		)
// 		if err != nil {
// 			log.Printf("GetUserFullProfile: Failed to scan achievement: %v", err)
// 			continue
// 		}
// 		achievementsList = append(achievementsList, a)
// 	}

// 	response := &user.UserProfileResponse{
// 		User:         targetUser,
// 		Stats:        userStats,
// 		WeeklyStats:  weeklyStats,
// 		Achievements: achievementsList,
// 		IsFriend:     isFriend,
// 	}

// 	log.Printf("GetUserFullProfile: Successfully fetched full profile for user %s", targetUserID)
// 	return response, nil
// }

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

func (s *UserService) AddDrinking(ctx context.Context, clerkID string, drankToday bool) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
	INSERT INTO daily_drinking (user_id, date, drank_today, logged_at)
	VALUES ($1, CURRENT_DATE, $2, NOW())
	ON CONFLICT (user_id, date) 
	DO UPDATE SET drank_today = $2, logged_at = NOW()
	`

	_, err = s.db.Exec(ctx, query, userID, drankToday)
	if err != nil {
		return fmt.Errorf("failed to log drinking: %w", err)
	}

	return nil
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
	// First, ensure pg_trgm extension is enabled (run this once in your migration)
	// CREATE EXTENSION IF NOT EXISTS pg_trgm;
	
	// Clean and prepare the query
	cleanQuery := strings.TrimSpace(query)
	searchPattern := "%" + cleanQuery + "%"
	
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
		-- Calculate similarity score (0-100%)
		GREATEST(
			-- Exact match bonus
			CASE 
				WHEN LOWER(username) = LOWER($2) THEN 100
				WHEN LOWER(email) = LOWER($2) THEN 100
				WHEN LOWER(first_name) = LOWER($2) THEN 95
				WHEN LOWER(last_name) = LOWER($2) THEN 95
				WHEN LOWER(CONCAT(first_name, ' ', last_name)) = LOWER($2) THEN 100
				ELSE 0
			END,
			-- Starts with bonus
			CASE 
				WHEN LOWER(username) LIKE LOWER($2) || '%' THEN 90
				WHEN LOWER(first_name) LIKE LOWER($2) || '%' THEN 85
				WHEN LOWER(last_name) LIKE LOWER($2) || '%' THEN 85
				ELSE 0
			END,
			-- Trigram similarity (PostgreSQL pg_trgm extension)
			GREATEST(
				similarity(username, $2) * 100,
				similarity(COALESCE(first_name, ''), $2) * 100,
				similarity(COALESCE(last_name, ''), $2) * 100,
				similarity(CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, '')), $2) * 100,
				similarity(COALESCE(email, ''), $2) * 60  -- Lower weight for email
			),
			-- Contains match (lower score)
			CASE 
				WHEN username ILIKE $1 THEN 50
				WHEN first_name ILIKE $1 THEN 45
				WHEN last_name ILIKE $1 THEN 45
				WHEN CONCAT(first_name, ' ', last_name) ILIKE $1 THEN 55
				WHEN email ILIKE $1 THEN 40
				ELSE 0
			END
		) AS similarity_score
	FROM users
	WHERE 
		-- Basic filter to reduce initial dataset
		(
			username ILIKE $1 OR
			email ILIKE $1 OR
			first_name ILIKE $1 OR
			last_name ILIKE $1 OR
			CONCAT(first_name, ' ', last_name) ILIKE $1 OR
			-- Trigram similarity threshold (adjust as needed, 0.1 = 10% similar)
			similarity(username, $2) > 0.1 OR
			similarity(COALESCE(first_name, ''), $2) > 0.1 OR
			similarity(COALESCE(last_name, ''), $2) > 0.1 OR
			similarity(CONCAT(COALESCE(first_name, ''), ' ', COALESCE(last_name, '')), $2) > 0.1
		)
		-- Optionally exclude the searching user
		AND clerk_id != $3
	ORDER BY 
		similarity_score DESC,
		-- Secondary sort by username for ties
		username
	LIMIT 50
	`

	rows, err := s.db.Query(ctx, sqlQuery, searchPattern, cleanQuery, clerkID)
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
		
		// Optional: You can store the similarity score in the user struct
		// if you want to display it in the UI
		// u.SimilarityScore = similarityScore
		
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
        -- Start with today or the most recent drinking day
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
