package eventsub

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AnonymousNotification struct {
	Subscription Subscription    `json:"subscription"`
	Event        json.RawMessage `json:"event"`
}

type RewardUpdateNotification struct {
	Subscription Subscription `json:"subscription"`
	Event        RewardUpdate `json:"event"`
}
type RewardAddNotification struct {
	Subscription Subscription `json:"subscription"`
	Event        RewardAdd    `json:"event"`
}
type ChannelFollowNotification struct {
	Subscription Subscription  `json:"subscription"`
	Event        ChannelFollow `json:"event"`
}

type Condition struct {
	BroadcasterUserID string `json:"broadcaster_user_id,omitempty"`
}

func appendCondition(sb *strings.Builder, value string) {
	if sb.Len() > 0 {
		sb.WriteString(", ")
	}
	sb.WriteString(value)
}

func (c Condition) String() string {
	sb := strings.Builder{}
	if c.BroadcasterUserID != "" {
		appendCondition(&sb, fmt.Sprintf("broadcaster_user_id: %q", c.BroadcasterUserID))
	}
	return sb.String()
}

type Subscription struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Version   string    `json:"version"`
	Status    string    `json:"status"`
	Cost      int       `json:"cost"`
	Condition Condition `json:"condition"`
	Transport struct {
		Method   string `json:"method"`
		Callback string `json:"callback"`
	} `json:"transport"`
	CreatedAt time.Time `json:"created_at"`
}

type RewardUpdate struct {
	ID                   string `json:"id"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	UserInput            string `json:"user_input"`
	Status               string `json:"status"`
	Reward               struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Cost   int    `json:"cost"`
		Prompt string `json:"prompt"`
	} `json:"reward"`
	RedeemedAt time.Time `json:"redeemed_at"`
}

type ChannelFollow struct {
	UserID               string    `json:"user_id"`
	UserLogin            string    `json:"user_login"`
	UserName             string    `json:"user_name"`
	BroadcasterUserID    string    `json:"broadcaster_user_id"`
	BroadcasterUserLogin string    `json:"broadcaster_user_login"`
	BroadcasterUserName  string    `json:"broadcaster_user_name"`
	FollowedAt           time.Time `json:"followed_at"`
}

type RewardAdd struct {
	ID                   string `json:"id"`
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	UserID               string `json:"user_id"`
	UserLogin            string `json:"user_login"`
	UserName             string `json:"user_name"`
	UserInput            string `json:"user_input"`
	Status               string `json:"status"`
	Reward               struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Cost   int    `json:"cost"`
		Prompt string `json:"prompt"`
	} `json:"reward"`
	RedeemedAt time.Time `json:"redeemed_at"`
}
