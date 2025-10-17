package services

import (
	"context"
	"errors"
	"fmt"
	"outDrinkMeAPI/internal/achievement"
	"outDrinkMeAPI/internal/calendar"
	"outDrinkMeAPI/internal/leaderboard"
	"outDrinkMeAPI/internal/stats"
	"outDrinkMeAPI/internal/user"
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

// InitSchema creates the users table with Clerk integration
// func (s *UserService) InitSchema(ctx context.Context) error {
// 	query := `
// 	CREATE TABLE IF NOT EXISTS users (
// 		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
// 		clerk_id VARCHAR(255) UNIQUE NOT NULL,
// 		email VARCHAR(255) UNIQUE NOT NULL,
// 		username VARCHAR(30) UNIQUE NOT NULL,
// 		first_name VARCHAR(100) NOT NULL,
// 		last_name VARCHAR(100) NOT NULL,
// 		image_url TEXT,
// 		email_verified BOOLEAN DEFAULT FALSE,
// 		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
// 		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
// 	);

// 	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
// 	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
// 	CREATE INDEX IF NOT EXISTS idx_users_clerk_id ON users(clerk_id);
// 	`

// 	_, err := s.db.Exec(ctx, query)
// 	return err
// }

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
	SELECT id, clerk_id, email, username, first_name, last_name, image_url, email_verified, created_at, updated_at
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
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (s *UserService) UpdateProfileByClerkID(ctx context.Context, clerkID string, req *user.UpdateProfileRequest) (*user.User, error) {
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

	user := &user.User{}
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
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	query := `
	SELECT 
		COALESCE(dd_today.drank_today, false) as today_status,
		COALESCE(COUNT(DISTINCT dd_week.date) FILTER (WHERE dd_week.drank_today = true), 0) as days_this_week,
		COALESCE(COUNT(DISTINCT dd_month.date) FILTER (WHERE dd_month.drank_today = true), 0) as days_this_month,
		COALESCE(COUNT(DISTINCT dd_year.date) FILTER (WHERE dd_year.drank_today = true), 0) as days_this_year,
		COALESCE(COUNT(DISTINCT dd_all.date) FILTER (WHERE dd_all.drank_today = true), 0) as total_days_drank,
		COALESCE(s.current_streak, 0) as current_streak,
		COALESCE(s.longest_streak, 0) as longest_streak,
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
	LEFT JOIN streaks s ON u.id = s.user_id
	LEFT JOIN weekly_stats ws ON u.id = ws.user_id
	LEFT JOIN user_achievements ua ON u.id = ua.user_id
	LEFT JOIN friendships f ON u.id = f.user_id
	WHERE u.id = $1
	GROUP BY u.id, dd_today.drank_today, s.current_streak, s.longest_streak
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

	// Get rank
	rankQuery := `
	SELECT rank FROM (
		SELECT 
			u.id,
			RANK() OVER (ORDER BY COALESCE(ws.days_drank, 0) DESC) as rank
		FROM users u
		LEFT JOIN weekly_stats ws ON u.id = ws.user_id
			AND ws.week_start = DATE_TRUNC('week', CURRENT_DATE)::DATE
		WHERE u.is_active = true
	) ranked
	WHERE id = $1
	`

	err = s.db.QueryRow(ctx, rankQuery, userID).Scan(&stats.Rank)
	if err != nil {
		stats.Rank = 0 // Default if no rank found
	}

	return stats, nil
}