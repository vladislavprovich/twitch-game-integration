package webhook

import (
	"time"

	"github.com/kirides/twitch-integration/twitch/eventsub"
)

type SubscriptionInfo struct {
	Type      string             `json:"type"`
	Version   string             `json:"version"`
	Condition eventsub.Condition `json:"condition"`
	Transport Transport          `json:"transport"`
}

type Transport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type SubscriptionResponse struct {
	Data         []Data `json:"data"`
	Total        int    `json:"total"`
	TotalCost    int    `json:"total_cost"`
	MaxTotalCost int    `json:"max_total_cost"`
}

type Data struct {
	ID        string             `json:"id"`
	Status    string             `json:"status"`
	Type      string             `json:"type"`
	Version   string             `json:"version"`
	Cost      int                `json:"cost"`
	Condition eventsub.Condition `json:"condition"`
	Transport TransportSlim      `json:"transport"`
	CreatedAt time.Time          `json:"created_at"`
}

type TransportSlim struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
}

type callbackVerificationData struct {
	Message struct {
		ID        string
		Retry     int
		Type      string
		Signature string
		Timestamp string
	}
	Subscription struct {
		Type    string
		Version string
	}
}
