package services

import (
	"context"
	"fmt"
	"log"
	sidequest "outDrinkMeAPI/internal/types/side_quest"
	"strings"

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

// ! Chekc if it works
func (s *SideQuestService) GetSideQuestBoard(ctx context.Context, clerkID string) (map[string][]sidequest.SideQuest, error) {
	result := make(map[string][]sidequest.SideQuest)
	result["buddies"] = []sidequest.SideQuest{}
	result["random"] = []sidequest.SideQuest{}

	var userID string
	err := s.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		log.Printf("Error finding user ID: %v", err)
		return nil, err
	}

	queryUserFriends := `
		SELECT friend_id FROM friendships WHERE user_id = $1 AND status = 'accepted'
		UNION
		SELECT user_id FROM friendships WHERE friend_id = $1 AND status = 'accepted'
	`

	rows, err := s.db.Query(ctx, queryUserFriends, userID)
	if err != nil {
		log.Printf("Error fetching friends: %v", err)
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

	if len(friendIDs) > 0 {
		placeholders := make([]string, len(friendIDs))
		args := make([]interface{}, len(friendIDs))
		for i, id := range friendIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}

		queryFriendSideQuests := fmt.Sprintf(`
			SELECT id, user_id, username, user_image_url, title, description, reward_amount, is_locked, is_public, is_anonymous, status, expires_at,created_at,submission_count
			FROM side_quests 
			WHERE user_id IN (%s) 
			ORDER BY created_at DESC 
			LIMIT 20`,
			strings.Join(placeholders, ","))

		fRows, err := s.db.Query(ctx, queryFriendSideQuests, args...)
		if err == nil {
			defer fRows.Close()
			for fRows.Next() {
				var q sidequest.SideQuest
				if err := fRows.Scan(&q.ID, &q.IssuerID, &q.IssuerName, &q.IssuerImage, &q.Title, &q.Description, &q.RewardAmount, &q.IsLocked, &q.IsPublic, &q.IsAnonymous, &q.Status, &q.ExpiresAt, &q.CreatedAt, &q.SubmissionCount); err == nil {
					result["buddies"] = append(result["buddies"], q)
				}
			}
		} else {
			log.Printf("Error fetching friend quests: %v", err)
		}
	}

	// ========================================================================
	// STEP 4: Get Random Quests posted by others (LIMIT 100)
	// ========================================================================

	// Exclude user and their friends
	excludedIDs := append(friendIDs, userID)

	placeholdersRand := make([]string, len(excludedIDs))
	argsRand := make([]interface{}, len(excludedIDs))
	for i, id := range excludedIDs {
		placeholdersRand[i] = fmt.Sprintf("$%d", i+1)
		argsRand[i] = id
	}

	queryRandom := fmt.Sprintf(`
		SELECT id, user_id, username, user_image_url, title, description, reward_amount, is_locked, is_public, is_anonymous, status, expires_at,created_at,submission_count
		FROM side_quest 
		WHERE user_id NOT IN (%s) 
		ORDER BY RANDOM() 
		LIMIT 100`,
		strings.Join(placeholdersRand, ","))

	rRows, err := s.db.Query(ctx, queryRandom, argsRand...)
	if err == nil {
		defer rRows.Close()
		for rRows.Next() {
			var q sidequest.SideQuest
			if err := rRows.Scan(&q.ID, &q.IssuerID, &q.IssuerName, &q.IssuerImage, &q.Title, &q.Description, &q.RewardAmount, &q.IsLocked, &q.IsPublic, &q.IsAnonymous, &q.Status, &q.ExpiresAt, &q.CreatedAt, &q.SubmissionCount); err == nil {
				// FIX: Append to slice (removed logical check on map key)
				result["random"] = append(result["random"], q)
			}
		}
	} else {
		log.Printf("Error fetching random quests: %v", err)
	}

	return result, nil
}

// func (s *SideQuestService) PostNewSideQuest(ctx context.Context, clerkID string, title string, description string, reward int, expiresAt time.Time, isPublic bool, iAnonymously bool) (map[string][]*store.Item, error) {
// 	var userID string
// 	var userGems int
// 	err := s.db.QueryRow(ctx, "SELECT id,gems FROM users WHERE clerk_id = $1", clerkID).Scan(&userID, &userGems)
// 	if err != nil {
// 		log.Printf("Error finding user ID: %v", err)
// 		return nil, err
// 	}

// 	// db call(check if user has gems to offer a reward)

// 	// db creation call
// 	// friends notificaion

// }

// func (s *SideQuestService) PostCompletion(ctx context.Context, clerkID string, title string, description string, reward int, expiresAt time.Time, isPublic bool, iAnonymously bool) (map[string][]*store.Item, error) {
// 	// db create call
// 	// notificaion to sidequest issuer to check it
// }
