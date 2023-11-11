package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"log/slog"

	"github.com/kirides/twitch-integration/twitch"
	"github.com/kirides/twitch-integration/twitch/irc"
)

type ChatMessage struct {
	Text    string `json:"text"`
	Sender  string `json:"sender"`
	Channel string `json:"channel"`
}

func handleChat(ctx context.Context, cnf twitchCnf, logger *slog.Logger, ew eventPublisher) error {
	logger = logger.With(logKeyCategory, "chat")

	if !cnf.ChatIntegration {
		logger.Info("integration disabled by configuration.")
		return nil
	}

	if cnf.OAuthToken == "" {
		logger.Info("No credentials. Integration disabled.")
		return nil
	}
	prefix := cnf.CommandPrefix
	logger.Info("Starting Twitch chat integration")

	c, err := irc.New(cnf.OAuthToken)
	if err != nil {
		return err
	}

	c.OnMessage(func(msg *irc.Message) error {
		logger.Debug("message received", slog.String("trailer", msg.Trailer))
		if !strings.HasPrefix(msg.Trailer, prefix) {
			return nil
		}
		if len(msg.Trailer) > 40 {
			msg.Trailer = msg.Trailer[:40]
		}

		data, err := json.Marshal(EventEnvelop{Type: "chat", Data: ChatMessage{Text: msg.Trailer, Sender: msg.Sender, Channel: msg.Channel}})
		if err != nil {
			logger.Error("could not serialize message", slog.Any("err", err))
			return nil
		}
		ew.Publish(data)
		return nil
	})

	c.OnError = func(format string, args ...interface{}) {
		logger.Error(fmt.Sprintf(format, args...))
	}
	rxPASS := regexp.MustCompile(`\bPASS\s+oauth:(.+?)\b`)
	c.OnSend = func(command string) {
		command = rxPASS.ReplaceAllLiteralString(command, "PASS oauth:***")
		logger.Debug("sent", slog.String("message", command))
	}
	c.OnReceived = func(command string) {
		logger.Debug("received", slog.String("message", command))
	}

	for {
		if err := c.OpenContext(ctx, twitch.IRCWebSocketURL); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			logger.Error("failed to connect", slog.Any("err", err))
			continue
		}
		defer c.Close()
		logger.Info("connected")
		break
	}

	for {
		if err := c.JoinContext(ctx, cnf.Channel); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			logger.Error("failed to join", slog.Any("err", err))
			continue
		}
		logger = logger.With(slog.String("channel", cnf.Channel))
		logger.Info("Joined")
		break
	}
	<-ctx.Done()

	return nil
}
