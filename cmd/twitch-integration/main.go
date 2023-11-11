package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/Microsoft/go-winio"
	"go.uber.org/zap"
)

func main() {
	// DLL
	// ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer cancel()
	// cnf, _ := readConfigJson()
	// handleEventPipe(ctx, cnf, log.Default())
}

func handleEventPipe(ctx context.Context, app *App, logger *zap.Logger) error {
	conn, err := winio.DialPipeAccess(ctx, `\\.\pipe\__TwitchIntegration_Kirides_Conn`, syscall.GENERIC_READ)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	logger.Debug("connected to event pipe")

	buffer := make([]byte, 16384)
	for {
		if _, err := io.ReadFull(conn, buffer[:2]); err != nil {
			return fmt.Errorf("could not read message size. %w", err)
		}
		size := binary.LittleEndian.Uint16(buffer[:2])
		if int(size) > len(buffer) {
			return fmt.Errorf("unexpectedly large message")
		}
		if _, err := io.ReadFull(conn, buffer[:size]); err != nil {
			return fmt.Errorf("could not read message. %w", err)
		}
		type envelope struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		var event envelope
		if err := json.Unmarshal(buffer[:size], &event); err != nil {
			return fmt.Errorf("could not deserialize message. %w", err)
		}

		cnf := app.GetConfig()
		switch event.Type {
		case "chat":
			type ChatMessage struct {
				Text    string `json:"text"`
				Sender  string `json:"sender"`
				Channel string `json:"channel"`
			}
			var chatMessage ChatMessage
			if err := json.Unmarshal(event.Data, &chatMessage); err != nil {
				return fmt.Errorf("could not deserialize chat message event. %w", err)
			}
			if fn, ok := cnf.Twitch.Chat[chatMessage.Text]; ok {
				logger.Info("Event accepted", zap.String("sender", chatMessage.Sender), zap.String("text", chatMessage.Text), zap.Strings("actions", fn.Actions))
				for _, fn := range fn.Actions {
					fn := strings.TrimSpace(fn)
					enqueueEvent(fmt.Sprintf("CHAT %s %s", chatMessage.Sender, fn))
				}
			}
		case "redemption":
			type Redemption struct {
				Title    string `json:"title"`
				Redeemer string `json:"redeemer"`
				Channel  string `json:"channel"`
			}
			var redeption Redemption
			if err := json.Unmarshal(event.Data, &redeption); err != nil {
				return fmt.Errorf("could not deserialize redeption event. %w", err)
			}
			logger.Debug("reward triggered", zap.String("redeemer", redeption.Redeemer), zap.String("reward", redeption.Title))
			if fn, ok := cnf.Twitch.Rewards[strings.ToUpper(redeption.Title)]; ok {
				logger.Info("handling reward", zap.String("redeemer", redeption.Redeemer), zap.String("reward", redeption.Title), zap.Strings("actions", fn))
				for _, fn := range fn {
					fn := strings.TrimSpace(fn)
					enqueueEvent(fmt.Sprintf("REWARD_ADD %s %s", redeption.Redeemer, fn))
				}
			}
		case "bits":
			type BitsEvent struct {
				BitsUsed int    `json:"bitsUsed"`
				User     string `json:"user"`
				Channel  string `json:"channel"`
			}
			var redeption BitsEvent
			if err := json.Unmarshal(event.Data, &redeption); err != nil {
				return fmt.Errorf("could not deserialize redeption event. %w", err)
			}
			logger.Debug("bits triggered", zap.String("bits_user", redeption.User), zap.Int("bits", redeption.BitsUsed))
			if fn, ok := cnf.Twitch.Bits[redeption.BitsUsed]; ok {
				logger.Info("handling bits", zap.String("bits_user", redeption.User), zap.Int("bits", redeption.BitsUsed), zap.Strings("actions", fn))
				for _, fn := range fn {
					fn := strings.TrimSpace(fn)
					enqueueEvent(fmt.Sprintf("BITS_USED %s %s", redeption.User, fn))
				}
			}
		case "streamelements-perk":
			type Redemption struct {
				Title    string `json:"title"`
				Redeemer string `json:"redeemer"`
				Channel  string `json:"channel"`
			}
			var redeption Redemption
			if err := json.Unmarshal(event.Data, &redeption); err != nil {
				return fmt.Errorf("could not deserialize streamelements-perk event. %w", err)
			}
			logger.Debug("StreamElements perk triggered", zap.String("redeemer", redeption.Redeemer), zap.String("perk", redeption.Title))
			if fn, ok := cnf.StreamElements.Perks[strings.ToUpper(redeption.Title)]; ok {
				logger.Info("handling StreamElements perk", zap.String("redeemer", redeption.Redeemer), zap.String("perk", redeption.Title), zap.Strings("actions", fn))
				for _, fn := range fn {
					fn := strings.TrimSpace(fn)
					enqueueEvent(fmt.Sprintf("REWARD_ADD %s %s", redeption.Redeemer, fn))
				}
			}
		}
	}
}
