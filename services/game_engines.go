package services

type KingsCupLogic struct{}

func (g *KingsCupLogic) InitState() interface{} {
	return map[string]string{"status": "waiting_for_players"}
}

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	// Simple Echo for now, replace with real game logic later
	s.Broadcast <- msg 
}

type BurnBookLogic struct{}

func (g *BurnBookLogic) InitState() interface{} {
	return nil
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	s.Broadcast <- msg
}