package streamelements

import "time"

const (
	EventRedemption = "redemption"
)

type ItemType string

const (
	ItemPerk   ItemType = "perk"
	ItemEffect ItemType = "effect"
	ItemCode   ItemType = "code"
)

type Redemption struct {
	RedeemerType string        `json:"redeemerType"`
	Completed    bool          `json:"completed"`
	Input        []interface{} `json:"input"`
	ID           string        `json:"_id"`
	Channel      string        `json:"channel"`
	Redeemer     struct {
		ID        string `json:"_id"`
		Avatar    string `json:"avatar"`
		Username  string `json:"username"`
		Inactive  bool   `json:"inactive"`
		IsPartner bool   `json:"isPartner"`
	} `json:"redeemer"`
	Item struct {
		Alert struct {
			Graphics struct {
				Duration int    `json:"duration"`
				Src      string `json:"src"`
				Type     string `json:"type"`
			} `json:"graphics"`
			Audio struct {
				Volume float64     `json:"volume"`
				Src    interface{} `json:"src"`
			} `json:"audio"`
			Enabled bool `json:"enabled"`
		} `json:"alert"`
		UserInput []interface{} `json:"userInput"`
		ID        string        `json:"_id"`
		Name      string        `json:"name"`
		Type      ItemType      `json:"type"`
		Cost      int           `json:"cost"`
	} `json:"item"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
