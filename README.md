
# Introduction

The code is made up of two binaries, a DLL which is used directly by the game and a so called 'connector' executable which
runs side by side with the game.

The job of the connector is it to talk to twitch, streamelements, etc. and communicate events via `named pipes` to
the DLL.

This decision was/is based on reducing the chances of the game in question crashing due to malformed payloads
or any other kind of malfunction while communicating with a third party service.

## Building the source

The easy way to build the sources comprises of having a linux [docker](https://www.docker.com/) runtime accessible and
using the [build.sh](./build/docker/build.sh) file provided to setup a cross-compilation environment.
_This uses Mingw to cross compile_

The manual way, on Windows, is to install Go 1.24.x and [Zig](https://ziglang.org/)
then use [build-twitch-integration-zig.bat](./build/windows/build-twitch-integration-zig.bat)

## Using the twitch integration

_This code provided is only the bare shell that allows communication with twitch and StreamElements Servers
to get notified on chat messages, Perk and Reward redemptions. It's up to the game-specific implementation
to make use of the provided information._

### Setting up the connector

For the connector to work, you have to enter the neccessery credentials into the
`twitch-integration-connector.json` which will automatically be generated upon launching it for the first time.

<details>
<summary>Example <code>twitch-integration-connector.json</code></summary>

```json
{
  // enables more detailed log output, can contain sensitive data
  "debug": false,
  "twitch": {
    // name of the channel to join for chat commands
    "channel": "Channel name where commands will be sent",
    // the token with permissions for reading chat and channel point redemptions
    "oauth_token": "Get it here: https://id.twitch.tv/oauth2/authorize?response_type=token&client_id=1ab71yymdkcck627lsp93whxbmj0om&redirect_uri=https://twitchapps.com/tokengen/&scope=channel%3Aread%3Asubscriptions%20bits%3Aread%20channel%3Aread%3Aredemptions%20chat%3Aread",
    // enables listening to channelpoints redemptions
    "channel_points": true,
    // enables listening to chat messages
    "chat": true,
    "command_prefix": "#"
  },
  "streamElements": {
    // enables the StreamElements module
    "enabled": false,
    // the token allows to read perk redemptions, this includes buying thing in the "store"
    "token": "Your JWT token from https://streamelements.com/dashboard/account/channels 'Show secrets'",
    // the name of the channel which the token belongs to
    "channel": "Channel Name for token"
  }
}
```

</details>


### Setting up the Integration

<details>
<summary>Example <code>twitch-integration.json</code></summary>

```json
{
	"debug": false,
	"streamElements": {
	  "perks": {
		// A key-value pair of Perk and Actions
		// "Perk Name": "Game specific twitch integration action"
		"Item": ["TWI_SpawnItemRandom"],
		"Item1": ["TWI_SpawnRandomItemNoArmorWeapons"]
	  }
	},
	"twitch": {
	  "rewards": {
		// A key-value pair of Reward and Actions
		// "Reward Name": "Game specific twitch integration action"
		"Item": ["TWI_InvertKeyControls"],
		"Item1": ["TWI_SpawnRandomMonster 1"]
	  },
	  "chat": {
		// A collection of "chat-prefix" and associated data
		// "chat-prefix": {...}
		"#weak": {
		  // the Game specific twitch integration actions 
		  "actions": ["TWI_Weakest_Weapon"],
		  // the cooldown for the chat message
		  "cooldown_sec": 120,
		  // a message that will be sent after publishing the action
		  "message": ""
		},
		"#hp_1": {
		  "actions": ["TWI_SetHP 1"],
		  "cooldown_sec": 120,
		  "message": ""
		}
	  }
	}
  }
```

</details>
