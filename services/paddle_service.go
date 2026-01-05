package services

import (
	"github.com/PaddleHQ/paddle-go-sdk"
)

type PaddleService struct {
	PaddleClient *paddle.SDK
}

func NewPaddleService(PaddleClient *paddle.SDK) *PaddleService {
	return &PaddleService{PaddleClient: PaddleClient}
}
