package main

import (
	"context"
	"encoding/json"
	"strings"

	"log/slog"

	"slices"

	"github.com/kirides/twitch-integration/twitch"
	"github.com/kirides/twitch-integration/twitch/pubsub"
)

type Redemption struct {
	Title    string `json:"title"`
	Redeemer string `json:"redeemer"`
	Channel  string `json:"channel"`
}

type BitsEvent struct {
	BitsUsed int    `json:"bitsUsed"`
	User     string `json:"user"`
	Channel  string `json:"channel"`
}

func handlePubSub(ctx context.Context, logger *slog.Logger, cnf twitchCnf, broker eventPublisher) {
	logger = logger.With(logKeyCategory, "pubsub")

	if !(cnf.ChannelPointsIntegration || cnf.BitsIntegration) {
		logger.Info("integration disabled by configuration.")
		return
	}

	if cnf.OAuthToken == "" {
		return
	}

	logger.Info("Starting Twitch Channelpoints integration.")

	resp, err := twitch.OAuth2Validate(context.Background(), cnf.OAuthToken)
	if err != nil {
		logger.Error("Could not validate OAuth Token", slog.Any("err", err))
		return
	}

	subFns := []func(conn *pubsub.Connection){}

	if cnf.ChannelPointsIntegration {
		if found := slices.Contains(resp.Scopes, "channel:read:redemptions"); !found {
			logger.Error("OAuth token does not contain required scope(s)", slog.String("err", "scope not satisfied"), slog.String("scope", "channel:read:redemptions"))
			return
		}
		subFns = append(subFns, func(conn *pubsub.Connection) {

			conn.Handle(pubsub.TopicRewardRedeemed, pubsub.H(func(rr *pubsub.RewardRedeemed) {
				logger.Info("Reward redeemed", slog.Any("Reward", rr))

				userWithoutSpaces := rr.Redemption.User.Login
				userWithoutSpaces = strings.Replace(userWithoutSpaces, " ", "", -1)

				redemption := rr.Redemption.Reward.Title
				data, err := json.Marshal(EventEnvelop{Type: "redemption", Data: Redemption{Title: redemption, Redeemer: userWithoutSpaces, Channel: "-"}})
				if err != nil {
					logger.Error("could not serialize redemption", slog.Any("err", err), slog.String("redeeming_user", rr.Redemption.User.Login))
					return
				}
				logger.Debug("Reward redeemed", slog.String("redeeming_user", rr.Redemption.User.Login), slog.String("reward", redemption))
				broker.Publish(data)
			}))

			if err := conn.Sub(ctx, cnf.OAuthToken, pubsub.TopicChannelPoints); err != nil {
				logger.Error("failed to subscribe to channelpoints topic.", slog.Any("err", err))
			} else {
				logger.Info("subscribed to channelpoints")
			}
		})
	}

	if cnf.BitsIntegration {
		if found := slices.Contains(resp.Scopes, "bits:read"); !found {
			logger.Error("OAuth token does not contain required scope(s)", slog.String("err", "scope not satisfied"), slog.String("scope", "bits:read"))
			return
		}

		subFns = append(subFns, func(conn *pubsub.Connection) {
			conn.Handle(pubsub.TopicBitsEvent, pubsub.H(func(rr *pubsub.BitsEvent) {
				logger.Info("Bits used", slog.Any("BitsEvent", rr))

				userWithoutSpaces := rr.UserName
				userWithoutSpaces = strings.Replace(userWithoutSpaces, " ", "", -1)

				data, err := json.Marshal(EventEnvelop{Type: "bits", Data: BitsEvent{BitsUsed: rr.BitsUsed, User: userWithoutSpaces, Channel: rr.ChannelName}})
				if err != nil {
					logger.Error("could not serialize bits", slog.Any("err", err), slog.String("user", rr.UserName))
					return
				}
				logger.Debug("Reward redeemed", slog.String("user", rr.UserName), slog.Int("bits", rr.BitsUsed))
				broker.Publish(data)
			}))

			if err := conn.Sub(ctx, cnf.OAuthToken, pubsub.TopicBitsEvents); err != nil {
				logger.Error("failed to subscribe to bits-events topic.", slog.Any("err", err))
			} else {
				logger.Info("subscribed to bits-events")
			}
		})
	}

	conn, err := pubsub.Connect(ctx, logger)
	if err != nil {
		logger.Error("failed to connect to twitch pubsub.", slog.Any("err", err))
		return
	}
	defer conn.Close()

	conn.OnEvent = func(e pubsub.Event) {
		logger.Debug("EVENT RECEIVED", slog.String("type", e.Type), slog.String("data", string(e.Data)), slog.String("error", string(e.Error)))
	}

	go func() {
		if err := conn.ProcessEvents(ctx); err != nil {
			logger.Error("failed to process events.", slog.Any("err", err))
			return
		}
	}()

	for _, v := range subFns {
		v(conn)
	}

	<-ctx.Done()
}
