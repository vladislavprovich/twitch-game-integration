package twitch

import "net/url"

const (
	IRCWebSocketURL   = "wss://irc-ws.chat.twitch.tv:443"
	PubSubURL         = "wss://pubsub-edge.twitch.tv"
	OAuth2ValidateURL = "https://id.twitch.tv/oauth2/validate"
)

func QueryOAuth2TokenURL(values url.Values) string {
	return "https://id.twitch.tv/oauth2/token?" + values.Encode()
}

func QueryUsersURL(values url.Values) string {
	return "https://api.twitch.tv/helix/users?" + values.Encode()
}

// aaaa
func QuerySubscriptionsURL(values url.Values) string {
	return "https://api.twitch.tv/helix/eventsub/subscriptions?" + values.Encode()
}
