package handlers

import (
	"outDrinkMeAPI/services"
)

type DrinkingGamesHandler struct {
	drinkingGamesService *services.DrinkingGamesService
}

func NewDrinkingGamesHandler(drinkingGamesService *services.DrinkingGamesService) *DrinkingGamesHandler {
	return &DrinkingGamesHandler{
		drinkingGamesService: drinkingGamesService,
	}
}
