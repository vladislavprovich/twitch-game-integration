package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"log/slog"

	"slices"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/kirides/twitch-integration/twitch"
)

type request struct {
	Type  string      `json:"type"`
	Nonce string      `json:"nonce"`
	Data  interface{} `json:"data"`
}

type requestHolder struct {
	Request    request
	responseCh chan Event
}

type Connection struct {
	messages       chan Event
	conn           *websocket.Conn
	reconnectTimer *time.Timer
	topics         []string

	tokensByChannel  map[string]OauthToken
	pendingResponses map[string]requestHolder
	handlers         map[MessageTopic][]func(json.RawMessage)

	http         *http.Client
	Logger       *slog.Logger
	readCtx      context.Context
	cancelReadFn context.CancelFunc

	OnEvent func(Event)

	wg sync.WaitGroup
}

type OauthToken struct {
	Token      string
	Validation twitch.OAuth2ValidateResponse
}

type Printer interface {
	Printf(fmt string, args ...interface{})
}

type Event struct {
	Type  string          `json:"type"`
	Error string          `json:"error"`
	Nonce string          `json:"nonce"`
	Data  json.RawMessage `json:"data"`
}

func Connect(ctx context.Context, logger *slog.Logger) (*Connection, error) {
	readCtx, cancelFn := context.WithCancel(context.Background())
	c := &Connection{
		conn:             nil,
		reconnectTimer:   time.NewTimer(10 * time.Second),
		messages:         make(chan Event, 1),
		tokensByChannel:  make(map[string]OauthToken),
		handlers:         make(map[MessageTopic][]func(json.RawMessage)),
		pendingResponses: make(map[string]requestHolder),

		http:         http.DefaultClient,
		Logger:       logger,
		readCtx:      readCtx,
		cancelReadFn: cancelFn,
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

func newListenRequest(token string, topics ...string) (request, string) {
	type listenData struct {
		Topics    []string `json:"topics"`
		AuthToken string   `json:"auth_token"`
	}

	req := request{
		Type:  "LISTEN",
		Nonce: uuid.NewString(),
		Data: listenData{
			Topics:    topics,
			AuthToken: token,
		},
	}

	return req, req.Nonce
}

func (c *Connection) onEvent(e Event) {
	if c.OnEvent != nil {
		c.OnEvent(e)
	}
}
func (c *Connection) connect(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, twitch.PubSubURL, &websocket.DialOptions{
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		return err
	}
	c.conn = conn
	c.stopReconnectTimer()

	c.wg.Add(1)
	go func(c *Connection) {
		defer c.wg.Done()
		for {
			t, d, err := c.conn.Read(c.readCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				c.Logger.Error("Failed to read message", slog.Any("err", err))
				return
			}
			if t != websocket.MessageText {
				c.Logger.Warn("Unsupported messagetype", "messageType", t.String())
				continue
			}
			var evt Event
			if err := json.Unmarshal(d, &evt); err != nil {
				c.Logger.Warn("could not unmarshal message", slog.Any("err", err))
				continue
			}
			c.messages <- evt
		}
	}(c)
	return nil
}

func (c *Connection) Close() error {
	c.cancelReadFn()
	c.wg.Wait()
	c.readCtx, c.cancelReadFn = context.WithCancel(context.Background())
	return nil
}

func (c *Connection) write(ctx context.Context, req request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (c *Connection) reconnect(ctx context.Context) error {
	c.conn.Close(websocket.StatusNormalClosure, "reconnecting")

	if err := c.connect(ctx); err != nil {
		return err
	}

	var chans []chan Event

	for _, v := range c.tokensByChannel {
		req, nonce := newListenRequest(v.Token, c.topics...)
		responseCh := make(chan Event)
		c.pendingResponses[nonce] = requestHolder{req, responseCh}
		chans = append(chans, responseCh)
		if err := c.write(ctx, req); err != nil {
			return err
		}
	}

	for _, c := range chans {
		select {
		case <-c:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

type TopicType int

const (
	TopicChannelPoints TopicType = iota
	TopicChannel
	TopicBitsEvents
)

func (t TopicType) String() string {
	switch t {
	case TopicChannelPoints:
		return "TopicChannelPoints"
	}
	return "Unknown"
}

func isScopeSupported(scope string, scopes []string) bool {
	return slices.Contains(scopes, scope)
}

func (c *Connection) Sub(ctx context.Context, oauthToken string, topicType TopicType) error {
	resp, err := twitch.OAuth2Validate(ctx, oauthToken)
	if err != nil {
		return err
	}
	c.tokensByChannel[resp.UserID] = OauthToken{Token: oauthToken, Validation: resp}

	var topic string
	switch {
	case topicType == TopicChannelPoints && isScopeSupported("channel:read:redemptions", resp.Scopes):
		topic = fmt.Sprintf("channel-points-channel-v1.%s", resp.UserID)
	case topicType == TopicBitsEvents && isScopeSupported("bits:read", resp.Scopes):
		topic = fmt.Sprintf("channel-bits-events-v2.%s", resp.UserID)
	default:
		return fmt.Errorf("topic not supported %q. Maybe missing scope in OAuth token", topicType)
	}

	req, nonce := newListenRequest(oauthToken, topic)
	responseCh := make(chan Event)
	c.pendingResponses[nonce] = requestHolder{req, responseCh}

	if err := c.write(ctx, req); err != nil {
		return err
	}
	c.topics = append(c.topics, topic)
	select {
	case e := <-responseCh:
		if e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (c *Connection) stopReconnectTimer() {
	if !c.reconnectTimer.Stop() {
		select {
		case <-c.reconnectTimer.C:
		default:
		}
	}
}

func (c *Connection) ping(ctx context.Context) error {
	c.conn.Write(ctx, websocket.MessageText, []byte(`{"type":"PING"}`))
	c.reconnectTimer.Reset(10 * time.Second)

	return nil
}

func H[T any](handler func(*T)) func(json.RawMessage) {
	return func(rm json.RawMessage) {
		var data T
		if err := json.Unmarshal(rm, &data); err != nil {
			log.Printf("failed to unmarshal. %v", err)
			return
		}
		handler(&data)
	}
}

type MessageTopic string

const (
	TopicRewardRedeemed MessageTopic = "reward-redeemed"
	TopicBitsEvent      MessageTopic = "bits_event"
)

func (c *Connection) Handle(typ MessageTopic, handler func(json.RawMessage)) {
	existing := c.handlers[typ]
	existing = append(existing, handler)
	c.handlers[typ] = existing
}

func (c *Connection) ProcessEvents(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	if err := c.ping(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-c.reconnectTimer.C:
			if err := c.reconnect(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("reconnect failed. %w", err)
			}
		case <-ticker.C:
			if err := c.ping(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
		case e := <-c.messages:
			if req, ok := c.pendingResponses[e.Nonce]; ok {
				req.responseCh <- e
				close(req.responseCh)
			}
			c.onEvent(e)
			if e.Type == "RECONNECT" {
				if err := c.reconnect(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					return fmt.Errorf("reconnect failed. %w", err)
				}
			} else if e.Type == "PONG" {
				c.stopReconnectTimer()
			} else if e.Type == "MESSAGE" {
				var data eventData
				c.Logger.Debug("message received", "message", e)
				if err := json.Unmarshal(e.Data, &data); err != nil {
					c.Logger.Warn("failed to unmarshal message", slog.Any("err", err))
					continue
				}
				if slices.Contains(c.topics, data.Topic) {
					var handlerPayload json.RawMessage
					var handlerType string

					switch {
					case strings.HasPrefix(data.Topic, "channel-bits-events-v2."):
						var msg bitsEvent
						if err := json.Unmarshal([]byte(data.Message), &msg); err != nil {
							c.Logger.Warn("failed to unmarshal message", slog.Any("err", err))
							continue
						}
						handlerType = msg.MessageType
						handlerPayload = msg.Data
					// case strings.HasPrefix(data.Topic, "channel-points-channel-v1."):
					default:
						var msg Message
						if err := json.Unmarshal([]byte(data.Message), &msg); err != nil {
							c.Logger.Warn("failed to unmarshal message", slog.Any("err", err))
							continue
						}
						handlerType = msg.Type
						handlerPayload = msg.Data
					}

					handlers := c.handlers[MessageTopic(handlerType)]
					for _, v := range handlers {
						v(handlerPayload)
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

type eventData struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type bitsEvent struct {
	Data        json.RawMessage `json:"data"`
	Version     string          `json:"version"`
	MessageType string          `json:"message_type"`
	MessageID   string          `json:"message_id"`
}
