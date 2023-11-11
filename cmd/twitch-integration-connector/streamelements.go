package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"log/slog"

	socketio "github.com/kirides/socketio-client"
	"github.com/kirides/twitch-integration/streamelements"
)

func handleStreamElements(ctx context.Context, cnf streamElementsCnf, logger *slog.Logger, ew eventPublisher) error {
	logger = logger.With(slog.String(logKeyCategory, "streamElements"))
	if !cnf.Enabled {
		logger.Info("integration disabled by configuration.")
		return nil
	}

	if cnf.Token == "" {
		logger.Info("No credentials. Integration disabled.")
		return nil
	}
	logger.Info("Starting StreamElements integration.")

	sio := socketio.New(loggerFn(func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...))
	}))
	if err := sio.Connect(ctx, `wss://realtime.streamelements.com/socket.io/`); err != nil {
		return err
	}
	defer sio.Close()

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		sio.Close()
	}()

	type authRequest struct {
		Method string `json:"method"`
		Token  string `json:"token"`
	}
	authReq := authRequest{Method: "jwt", Token: cnf.Token}
	if err := sio.SendMessage(ctx, socketio.ActionEVENT, "authenticate", authReq); err != nil {
		return fmt.Errorf("failed to send authenticate request. %w", err)
	}
	logger.Info("authenticated")

	sioBroker := socketio.NewBroker(sio)

	handler := make(chan *socketio.Message, 5)
	sioBroker.Subscribe(streamelements.EventRedemption, handler)
	defer sioBroker.Unsubscribe(streamelements.EventRedemption, handler)

	wg.Add(1)
	go func() {
		defer wg.Done()
		sioBroker.Listen(ctx)
	}()

	streamElementsConsumeLoop(ctx, logger, handler, cnf, ew)
	wg.Wait()
	return nil
}

func streamElementsConsumeLoop(ctx context.Context, logger *slog.Logger, handler <-chan *socketio.Message, cnf streamElementsCnf, ew eventPublisher) {
	logger = logger.With(slog.String("channel", cnf.Channel))
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-handler:
			var red streamelements.Redemption

			logger.Debug("Received redemption", slog.String("payload", string(msg.Payload)))
			if err := json.Unmarshal(msg.Payload, &red); err != nil {
				logger.Warn("redemption could not be deserialized", slog.Any("err", err))
				continue
			}

			if red.Item.Type == streamelements.ItemPerk {
				user := red.Redeemer.Username
				user = strings.Replace(user, " ", "", -1)

				redemption := red.Item.Name
				data, err := json.Marshal(EventEnvelop{Type: "streamelements-perk", Data: Redemption{Title: redemption, Redeemer: user, Channel: cnf.Channel}})
				if err != nil {
					logger.Error("could not serialize redemption", slog.Any("err", err), slog.String("redeeming_user", red.Redeemer.Username))
					continue
				}
				logger.Debug("perk redeemed", slog.String("redeeming_user", red.Redeemer.Username), slog.String("perk", redemption))
				ew.Publish(data)
			}
		}
	}
}
