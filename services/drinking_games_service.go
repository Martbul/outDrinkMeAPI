package services

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type DrinkingGamesService struct {
	db *pgxpool.Pool
}

func NewDrinkingGamesService(db *pgxpool.Pool) *UserService {
	return &UserService{
		db: db,
	}
}
