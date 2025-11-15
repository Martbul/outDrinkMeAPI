package collection

type AlcoholItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	ImageUrl *string `json:"image_url"`
	Rarity   string  `json:"rarity"`
	Abv      float32 `json:"abv"`
}

type AlcoholCollectionByType map[string][]AlcoholItem
