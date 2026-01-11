package wish

type WishItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type CreateWishRequest struct {
	Text string `json:"text"`
}