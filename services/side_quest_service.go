package services

import (
	"context"
	"fmt"
	"log"
	sidequest "outDrinkMeAPI/internal/types/side_quest"
	"outDrinkMeAPI/utils"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SideQuestService struct {
	db           *pgxpool.Pool
	notifService *NotificationService
}

func NewSideQuestService(db *pgxpool.Pool, notifService *NotificationService) *SideQuestService {
	return &SideQuestService{
		db:           db,
		notifService: notifService,
	}
}
func (s *SideQuestService) GetSideQuestBoard(ctx context.Context, clerkID string) (map[string][]sidequest.SideQuest, error) {
	result := make(map[string][]sidequest.SideQuest)
	// Initialize slices so JSON returns [] instead of null
	result["buddies"] = []sidequest.SideQuest{}
	result["random"] = []sidequest.SideQuest{}

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		log.Printf("Error finding user ID: %v", err)
		return nil, err
	}

	// 1. Get Friend IDs (Bidirectional) - This logic is GOOD
	queryUserFriends := `
		SELECT friend_id FROM friendships WHERE user_id = $1 AND status = 'accepted'
		UNION
		SELECT user_id FROM friendships WHERE friend_id = $1 AND status = 'accepted'
	`
	rows, err := s.db.Query(ctx, queryUserFriends, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friendIDs []string
	for rows.Next() {
		var fid string
		if err := rows.Scan(&fid); err == nil {
			friendIDs = append(friendIDs, fid)
		}
	}

	// ========================================================================
	// STEP 2: Get BUDDIES Quests
	// ========================================================================
	if len(friendIDs) > 0 {
		placeholders := make([]string, len(friendIDs))
		args := make([]interface{}, len(friendIDs))
		for i, id := range friendIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}

		// FIX: Joined users table to get username/image
		// FIX: Filtered by status = 'OPEN'
		queryFriendSideQuests := fmt.Sprintf(`
			SELECT 
				sq.id, sq.issuer_id, u.username, u.image_url, 
				sq.title, sq.description, sq.reward_amount, sq.is_locked, 
				sq.is_public, sq.is_anonymous, sq.status, sq.expires_at, 
				sq.created_at, sq.submission_count
			FROM side_quests sq
			INNER JOIN users u ON sq.issuer_id = u.id
			WHERE sq.issuer_id IN (%s)
			AND sq.status = 'OPEN' 
			ORDER BY sq.created_at DESC 
			LIMIT 20`,
			strings.Join(placeholders, ","))

		fRows, err := s.db.Query(ctx, queryFriendSideQuests, args...)
		if err != nil {
			log.Printf("Error fetching friend quests: %v", err)
			return nil, err
		}
		defer fRows.Close()

		for fRows.Next() {
			var q sidequest.SideQuest
			// Note: scanning u.username into IssuerName and u.image_url into IssuerImage
			if err := fRows.Scan(&q.ID, &q.IssuerID, &q.IssuerName, &q.IssuerImage, &q.Title, &q.Description, &q.RewardAmount, &q.IsLocked, &q.IsPublic, &q.IsAnonymous, &q.Status, &q.ExpiresAt, &q.CreatedAt, &q.SubmissionCount); err == nil {
				result["buddies"] = append(result["buddies"], q)
			}
		}
	}

	// ========================================================================
	// STEP 3: Get RANDOM Quests
	// ========================================================================

	// Exclude User + Friends
	excludedIDs := append(friendIDs, userID)

	placeholdersRand := make([]string, len(excludedIDs))
	argsRand := make([]interface{}, len(excludedIDs))
	for i, id := range excludedIDs {
		placeholdersRand[i] = fmt.Sprintf("$%d", i+1)
		argsRand[i] = id
	}

	// FIX: Added check for is_public = true
	// FIX: Added JOIN users
	// FIX: Corrected table name (side_quests)
	queryRandom := fmt.Sprintf(`
		SELECT 
			sq.id, sq.issuer_id, u.username, u.image_url, 
			sq.title, sq.description, sq.reward_amount, sq.is_locked, 
			sq.is_public, sq.is_anonymous, sq.status, sq.expires_at, 
			sq.created_at, sq.submission_count
		FROM side_quests sq
		INNER JOIN users u ON sq.issuer_id = u.id
		WHERE sq.issuer_id NOT IN (%s) 
		AND sq.is_public = true
		AND sq.status = 'OPEN'
		ORDER BY RANDOM() 
		LIMIT 100`,
		strings.Join(placeholdersRand, ","))

	rRows, err := s.db.Query(ctx, queryRandom, argsRand...)
	if err != nil {
		log.Printf("Error fetching random quests: %v", err)
		return nil, err
	}
	defer rRows.Close()

	for rRows.Next() {
		var q sidequest.SideQuest
		if err := rRows.Scan(&q.ID, &q.IssuerID, &q.IssuerName, &q.IssuerImage, &q.Title, &q.Description, &q.RewardAmount, &q.IsLocked, &q.IsPublic, &q.IsAnonymous, &q.Status, &q.ExpiresAt, &q.CreatedAt, &q.SubmissionCount); err == nil {
			result["random"] = append(result["random"], q)
		}
	}

	return result, nil
}

func (s *SideQuestService) PostNewSideQuest(ctx context.Context, clerkID string, title string, description string, reward int, expiresAt time.Time, isPublic bool, isAnonymous bool) (map[string][]*sidequest.SideQuest, error) {
	// 1. Begin Transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 2. Get User ID, Gems, and Username (Username is needed for the notification)
	var userID string
	var userGems int
	var userName string 
	
	// Added 'username' to the query so we can say "John posted a quest"
	err = tx.QueryRow(ctx, "SELECT id, gems, username FROM users WHERE clerk_id = $1 FOR UPDATE", clerkID).Scan(&userID, &userGems, &userName)
	if err != nil {
		log.Printf("Error finding user ID: %v", err)
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// 3. Validate Balance
	if reward > userGems {
		return nil, fmt.Errorf("insufficient funds: you have %d gems, but reward is %d", userGems, reward)
	}

	// 4. Deduct Gems
	_, err = tx.Exec(ctx, "UPDATE users SET gems = gems - $1 WHERE id = $2", reward, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct gems: %w", err)
	}

	// 5. Insert New Side Quest
	createPostSQL := `
		INSERT INTO side_quests (
			issuer_id, 
			title, 
			description, 
			reward_amount, 
			is_locked, 
			is_public, 
			is_anonymous, 
			status, 
			expires_at, 
			created_at, 
			submission_count
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), 0
		)
		RETURNING id, created_at, status
	`

	newQuest := &sidequest.SideQuest{
		IssuerID:     userID,
		IssuerName:   &userName, // Populating this pointer for the response
		Title:        title,
		Description:  description,
		RewardAmount: float64(reward),
		IsLocked:     true,
		IsPublic:     isPublic,
		IsAnonymous:  isAnonymous,
		ExpiresAt:    expiresAt,
		SubmissionCount: 0,
	}

	err = tx.QueryRow(ctx, createPostSQL,
		newQuest.IssuerID,
		newQuest.Title,
		newQuest.Description,
		newQuest.RewardAmount,
		newQuest.IsLocked,
		newQuest.IsPublic,
		newQuest.IsAnonymous,
		sidequest.QuestStatusOpen,
		newQuest.ExpiresAt,
	).Scan(&newQuest.ID, &newQuest.CreatedAt, &newQuest.Status)

	if err != nil {
		return nil, fmt.Errorf("failed to insert side quest: %w", err)
	}

	// 6. Commit Transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 7. Trigger Notifications (Async)
	// Only send if the quest is NOT anonymous and IS public (usually logic implies friends see it unless it's private)
	if !isAnonymous && s.notifService != nil {
		// Convert string ID to UUID for the notification system
		issuerUUID, parseErr := uuid.Parse(userID)
		
		if parseErr == nil {
			go func() {
				// We create a detached context because 'ctx' might be cancelled when the HTTP request finishes
				// utils.FriendPostedQuest handles the friend lookup and notification creation
				utils.FriendPostedQuest(s.db, s.notifService, issuerUUID, userName, newQuest)
			}()
		} else {
			log.Printf("Notification skipped: could not parse userID '%s' to UUID: %v", userID, parseErr)
		}
	}

	// 8. Return Result
	return map[string][]*sidequest.SideQuest{
		"quest": {newQuest},
	}, nil
}

// func (s *SideQuestService) PostCompletion(ctx context.Context, clerkID string, title string, description string, reward int, expiresAt time.Time, isPublic bool, iAnonymously bool) (map[string][]*store.Item, error) {
// 	// db create call
// 	// notificaion to sidequest issuer to check it
// }
