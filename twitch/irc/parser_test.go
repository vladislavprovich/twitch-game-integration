package irc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePingMessage(t *testing.T) {

	msg, err := parseIRCv3(`PING :tmi.twitch.tv`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "PING", msg.Command.Name)
	assert.Equal(t, "tmi.twitch.tv", msg.Command.Args[0])
}

func TestParseCommandWithoutArgs(t *testing.T) {

	msg, err := parseIRCv3(`:demo!demo@demo.tmi.twitch.tv COMMAND`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "COMMAND", msg.Command.Name)
}

func TestParseCommandSingleArg(t *testing.T) {

	msg, err := parseIRCv3(`:demo!demo@demo.tmi.twitch.tv COMMAND Arg`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "COMMAND", msg.Command.Name)
	assert.Equal(t, "Arg", msg.Command.Args[0])
}

func TestParsePrivMsg(t *testing.T) {

	msg, err := parseIRCv3(`:demo!demo@demo.tmi.twitch.tv PRIVMSG #channel :This is a sample message`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "PRIVMSG", msg.Command.Name)
	assert.Equal(t, "#channel", msg.Command.Args[0])
	assert.Equal(t, "This is a sample message", msg.Command.Args[1])
}

func assertTag(t *testing.T, tag ircTag, key, value string) {
	assert.Equal(t, key, tag.Key)
	assert.Equal(t, value, tag.Value)
}

func TestParsePrivMsgWithTag(t *testing.T) {

	msg, err := parseIRCv3(`@display-name=D :demo!demo@demo.tmi.twitch.tv PRIVMSG #channel :This is a sample message`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "PRIVMSG", msg.Command.Name)
	assert.Equal(t, "#channel", msg.Command.Args[0])
	assert.Equal(t, "This is a sample message", msg.Command.Args[1])

	assertTag(t, msg.Tags[0], "display-name", "D")
}

func TestParsePrivMsgWithTags(t *testing.T) {

	msg, err := parseIRCv3(`@color=#FF4500;display-name=Demo;emotes=;subscriber=0;turbo=0;user-type=mod;user_type=mod :demo!demo@demo.tmi.twitch.tv PRIVMSG #channel :This is a sample message`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "PRIVMSG", msg.Command.Name)
	assert.Equal(t, "#channel", msg.Command.Args[0])
	assert.Equal(t, "This is a sample message", msg.Command.Args[1])

	assertTag(t, msg.Tags[0], "color", "#FF4500")
	assertTag(t, msg.Tags[1], "display-name", "Demo")
	assertTag(t, msg.Tags[2], "emotes", "")
	assertTag(t, msg.Tags[3], "subscriber", "0")
	assertTag(t, msg.Tags[4], "turbo", "0")
	assertTag(t, msg.Tags[5], "user-type", "mod")
	assertTag(t, msg.Tags[6], "user_type", "mod")
}

func TestParsePrivMsgWithTagsEndingEmpty(t *testing.T) {

	msg, err := parseIRCv3(`@badge-info=;badges=premium/1;client-nonce=453f818ab240af3d5890d3380eb9246e;color=;display-name=Demo;emotes=;first-msg=0;flags=;id=f878d0c2-a973-40ca-865d-57cce6ec147b;mod=0;room-id=67027439;subscriber=0;tmi-sent-ts=1639767497984;turbo=0;user-id=521149409;user-type= :demo!demo@demo.tmi.twitch.tv PRIVMSG #channel :haha ;D`)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.Equal(t, "demo!demo@demo.tmi.twitch.tv", msg.Prefix)
	assert.Equal(t, "PRIVMSG", msg.Command.Name)
	assert.Equal(t, "#channel", msg.Command.Args[0])
	assert.Equal(t, "haha ;D", msg.Command.Args[1])

	assertTag(t, msg.Tags[0], "badge-info", "")
	assertTag(t, msg.Tags[1], "badges", "premium/1")
	assertTag(t, msg.Tags[2], "client-nonce", "453f818ab240af3d5890d3380eb9246e")
	assertTag(t, msg.Tags[3], "color", "")
	assertTag(t, msg.Tags[4], "display-name", "Demo")
	assertTag(t, msg.Tags[5], "emotes", "")
	assertTag(t, msg.Tags[6], "first-msg", "0")
	assertTag(t, msg.Tags[7], "flags", "")
	assertTag(t, msg.Tags[8], "id", "f878d0c2-a973-40ca-865d-57cce6ec147b")
	assertTag(t, msg.Tags[9], "mod", "0")
	assertTag(t, msg.Tags[10], "room-id", "67027439")
	assertTag(t, msg.Tags[11], "subscriber", "0")
	assertTag(t, msg.Tags[12], "tmi-sent-ts", "1639767497984")
	assertTag(t, msg.Tags[13], "turbo", "0")
	assertTag(t, msg.Tags[14], "user-id", "521149409")
	assertTag(t, msg.Tags[15], "user-type", "")
}
