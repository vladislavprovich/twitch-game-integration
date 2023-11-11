package main

type config struct {
	Debug          bool              `json:"debug"`
	Twitch         twitchCnf         `json:"twitch"`
	StreamElements streamElementsCnf `json:"streamElements"`
}

type streamElementsCnf struct {
	Enabled bool   `json:"enabled"`
	Token   string `json:"token"`
	Channel string `json:"channel"`
}

type twitchCnf struct {
	Channel                  string `json:"channel"`
	OAuthToken               string `json:"oauth_token"`
	ChannelPointsIntegration bool   `json:"channel_points"`
	BitsIntegration          bool   `json:"bits"`
	ChatIntegration          bool   `json:"chat"`
	CommandPrefix            string `json:"command_prefix"`
}

func defaultConfig() config {
	return config{
		Twitch: twitchCnf{
			OAuthToken:               "Get it here: https://id.twitch.tv/oauth2/authorize?response_type=token&client_id=1ab71yymdkcck627lsp93whxbmj0om&redirect_uri=https://twitchapps.com/tokengen/&scope=channel%3Aread%3Asubscriptions%20bits%3Aread%20channel%3Aread%3Aredemptions%20chat%3Aread",
			CommandPrefix:            "#",
			ChatIntegration:          true,
			ChannelPointsIntegration: true,
			BitsIntegration:          true,
			Channel:                  "Channel name where commands will be sent",
		},
		StreamElements: streamElementsCnf{
			Enabled: false,
			Token:   "Your JWT token from https://streamelements.com/dashboard/account/channels 'Show secrets'",
			Channel: "Channel Name for token",
		},
	}
}
