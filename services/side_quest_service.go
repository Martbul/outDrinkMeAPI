package services

import "github.com/jackc/pgx/v5/pgxpool"

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