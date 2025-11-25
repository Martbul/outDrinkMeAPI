package sidequest


type SideQuestBoard struct {
	Type        string       `json:"type"` // "FRIENDS" or "PUBLIC"
	Quests      []SideQuest  `json:"quests"`
	ActiveCount int          `json:"activeCount"`
}