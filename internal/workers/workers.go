package workers

// import (
// 	"context"
// 	"database/sql"
// 	"log"
// 	"time"
// )

// // StartCleanupWorker starts a background routine to clean up expired photo dumps
// func StartCleanupWorker(db *sql.DB) {
// 	// Create a ticker that ticks every 1 hour
// 	ticker := time.NewTicker(1 * time.Hour)

// 	go func() {
// 		for {
// 			select {
// 			case <-ticker.C:
// 				cleanupExpiredSessions(db)
// 			}
// 		}
// 	}()
// }

// func cleanupExpiredSessions(db *sql.DB) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
// 	defer cancel()

// 	log.Println("Starting cleanup of expired photo dumps...")

// 	// 1. Find sessions that expired more than X time ago (e.g., 24 hours grace period)
//     // We fetch the ID and the URL because we need to delete the cloud file first.
// 	query := `
// 		SELECT pd.id, img.image_url 
// 		FROM photo_dump pd
// 		JOIN photo_dump_images img ON pd.id = img.photo_dump_id
// 		WHERE pd.expires_at < NOW()
// 	`

// 	rows, err := db.QueryContext(ctx, query)
// 	if err != nil {
// 		log.Printf("Error querying expired sessions: %v", err)
// 		return
// 	}
// 	defer rows.Close()

//     // Map to group images by session so we can handle them structurally
// 	imagesToDelete := []string{}
//     sessionIdsToDelete := make(map[string]bool)

// 	for rows.Next() {
// 		var sessionId string
// 		var imageUrl string
// 		if err := rows.Scan(&sessionId, &imageUrl); err != nil {
// 			continue
// 		}
// 		imagesToDelete = append(imagesToDelete, imageUrl)
//         sessionIdsToDelete[sessionId] = true
// 	}

// 	// 2. DELETE FROM CLOUD STORAGE
// 	if len(imagesToDelete) > 0 {
// 		// Call your specific cloud provider function here
// 		// err := myS3Client.DeleteObjects(imagesToDelete)
//         log.Printf("Deleting %d images from Cloud Storage...", len(imagesToDelete))
// 	}

// 	// 3. DELETE FROM DATABASE
//     // Because we used ON DELETE CASCADE in SQL, deleting the main photo_dump row
//     // will automatically delete the participants and image rows.
// 	for id := range sessionIdsToDelete {
// 		_, err := db.ExecContext(ctx, "DELETE FROM photo_dump WHERE id = $1", id)
// 		if err != nil {
// 			log.Printf("Failed to delete session %s: %v", id, err)
// 		} else {
//             log.Printf("Deleted session %s and its data", id)
//         }
// 	}
// }