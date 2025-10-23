package services

import (

	"github.com/jackc/pgx/v5/pgxpool"
)

type DocService struct {
	db *pgxpool.Pool
}

func NewDocService(db *pgxpool.Pool) *DocService {
	return &DocService{db: db}
}