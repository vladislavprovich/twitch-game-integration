package irc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"slices"

	"github.com/coder/websocket"
	"github.com/kirides/twitch-integration/twitch"
	"golang.org/x/time/rate"
)

var ErrRateExceeded = errors.New("ratelimit exceeded")

type messageHandler chan *Message

const (
	defaultMessageBufferSize = 20
)

// Message ...
type Message struct {
	Tags    map[string]string
	Trailer string
	Sender  string
	Channel string
	Command string
	Prefix  string
	Args    []string
}

// ChatClient ...
type ChatClient struct {
	conn                    *websocket.Conn
	messageHandlers         map[messageHandler]struct{}
	doneListening           chan struct{}
	rateLimitOp             *rate.Limiter
	rateLimit               *rate.Limiter
	messageHandlersInternal map[messageHandler]struct{}
	Nick                    string
	token                   string
	defaultTimeout          time.Duration
	mtx                     sync.Mutex
	handlerMtx              sync.Mutex

	OnSend     func(command string)
	OnReceived func(command string)
	OnError    func(format string, args ...interface{})
}

// New ...
func New(token string) (*ChatClient, error) {
	token = strings.TrimPrefix(token, "oauth:")

	resp, err := twitch.OAuth2Validate(context.Background(), token)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}

	found := slices.Contains(resp.Scopes, "chat:read")
	if !found {
		return nil, fmt.Errorf("OAuth token does not contain %q scope", "chat:read")
	}

	return &ChatClient{
		Nick:                    resp.Login,
		token:                   token,
		messageHandlersInternal: make(map[messageHandler]struct{}),
		messageHandlers:         make(map[messageHandler]struct{}),
		doneListening:           make(chan struct{}, 1),
		defaultTimeout:          time.Second * 20,

		rateLimitOp: rate.NewLimiter(3, 1),                                // 3 per second
		rateLimit:   rate.NewLimiter(rate.Every(time.Millisecond*500), 1), // 0.5 per second
	}, nil
}

func (c *ChatClient) isOp(user, channel string) bool {
	return strings.EqualFold(user, channel)
}

// Add ...
func (c *ChatClient) rateLimited(channel string) bool {
	if c.isOp(c.Nick, channel) {
		if !c.rateLimitOp.Allow() {
			return true
		}
	} else if !c.rateLimit.Allow() {
		return true
	}
	return false
}

var noTimeout = time.Time{}

// Send
func (c *ChatClient) Send(channel, content string) error {
	if c.rateLimited(channel) {
		return ErrRateExceeded
	}
	err := c.conn.Write(context.TODO(), websocket.MessageText, []byte(fmt.Sprintf("PRIVMSG #%s :%s\n", channel, content)))

	return err
}

// SendTimeout
func (c *ChatClient) SendTimeout(channel, content string, timeout time.Duration) error {
	if c.rateLimited(channel) {
		return ErrRateExceeded
	}

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err := c.conn.Write(ctx, websocket.MessageText, []byte(fmt.Sprintf("PRIVMSG #%s :%s\n", channel, content)))
	return err
}

// OpenContext ...
func (c *ChatClient) OpenContext(ctx context.Context, url string) error {
	c.resetHandlers()

	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "")
	}
	wsc, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if err != nil {
		return err
	}
	// resp.Body.Close()

	c.conn = wsc

	hand := make(chan *Message, defaultMessageBufferSize)

	c.addHandlerInternal(hand)
	defer c.removeHandlerInternal(hand)
	c.listenContext(ctx)

	if err := c.send("CAP REQ :twitch.tv/tags"); err != nil {
		return err
	}

	if err := c.waitForCaps(hand, ctx, c.defaultTimeout); err != nil {
		return err
	}

	if err := c.send(fmt.Sprintf("PASS oauth:%s", c.token)); err != nil {
		return err
	}
	if err := c.send(fmt.Sprintf("NICK %s", c.Nick)); err != nil {
		return err
	}

	if err := c.waitForAuthentication(hand, ctx, c.defaultTimeout); err != nil {
		return fmt.Errorf("could not authenticate: %w", err)
	}

	return nil
}

func (c *ChatClient) waitForAuthentication(hand <-chan *Message, ctx context.Context, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg, ok := <-hand:
			if ok {
				if msg.Command == "376" {
					return nil
				}
				if msg.Command == "NOTICE" && strings.Contains(msg.Trailer, "failed") {
					return fmt.Errorf("received failed response. %s", msg.Trailer)
				}
			}

		case <-ctx.Done():
			return fmt.Errorf("waiting for connection response cancelled. %w", ctx.Err())
		case <-timer.C:
			return fmt.Errorf("waiting for connection response timeout exceeded")
		}
	}
}

func (c *ChatClient) waitForCaps(hand <-chan *Message, ctx context.Context, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case msg := <-hand:
			if msg.Command == "CAP" &&
				len(msg.Args) > 1 &&
				msg.Args[1] == "ACK" {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("waiting for CAP response cancelled. %w", ctx.Err())
		case <-timer.C:
			return fmt.Errorf("waiting for CAP response timeout exceeded")
		}
	}
}

// Close ...
func (c *ChatClient) Close() error {
	if c.conn == nil {
		return fmt.Errorf("no connection")
	}

	if err := c.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		return err
	}
	c.conn = nil
	timer := time.NewTimer(c.defaultTimeout)
	defer timer.Stop()

	select {
	case <-c.doneListening:
	case <-timer.C:
		return fmt.Errorf("waiting for listening go-routine failed")
	}

	return nil
}

// OnMessage ...
func (c *ChatClient) OnMessage(handler func(msg *Message) error) {
	hand := make(messageHandler, defaultMessageBufferSize)
	c.addHandler(hand)
	go func() {
		defer c.removeHandler(hand)
		for msg := range hand {
			if err := handler(msg); err != nil {
				return
			}
		}
	}()
}

func (c *ChatClient) resetHandlers() {
	c.handlerMtx.Lock()
	defer c.handlerMtx.Unlock()

	c.messageHandlersInternal = make(map[messageHandler]struct{})
	// c.messageHandlers = make(map[messageHandler]struct{})
}

func (c *ChatClient) addHandler(hand messageHandler) {
	c.handlerMtx.Lock()
	defer c.handlerMtx.Unlock()
	c.messageHandlers[hand] = struct{}{}
}

func (c *ChatClient) removeHandler(hand messageHandler) {
	c.handlerMtx.Lock()
	defer c.handlerMtx.Unlock()
	delete(c.messageHandlers, hand)
}

func (c *ChatClient) addHandlerInternal(hand messageHandler) {
	c.handlerMtx.Lock()
	defer c.handlerMtx.Unlock()
	c.messageHandlersInternal[hand] = struct{}{}

}
func (c *ChatClient) removeHandlerInternal(hand messageHandler) {
	c.handlerMtx.Lock()
	defer c.handlerMtx.Unlock()
	delete(c.messageHandlersInternal, hand)
}

func (c *ChatClient) send(txt string) error {
	if c.OnSend != nil {
		c.OnSend(txt)
	}
	c.mtx.Lock()
	defer c.mtx.Unlock()
	err := c.conn.Write(context.TODO(), websocket.MessageText, []byte(txt))
	return err
}

func (c *ChatClient) JoinContext(ctx context.Context, channel string) error {
	hand := make(messageHandler, defaultMessageBufferSize)
	c.addHandlerInternal(hand)
	defer c.removeHandlerInternal(hand)

	c.send(fmt.Sprintf("JOIN #%s\n", channel))

	timer := time.NewTimer(time.Second * 30)
	defer timer.Stop()

	expectedPrefix := fmt.Sprintf("%s!%s@%s.tmi.twitch.tv", c.Nick, c.Nick, c.Nick)

	for {
		select {
		case msg := <-hand:
			if msg.Prefix == expectedPrefix &&
				msg.Command == "JOIN" &&
				len(msg.Args) > 0 {
				// check proper channel
				if len(msg.Args[0]) > 1 {
					if msg.Args[0][0] == '#' {
						if msg.Args[0][1:] == channel {
							c.removeHandlerInternal(hand)
							return nil
						}
					}
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("JOIN cancelled. %w", ctx.Err())
		case <-timer.C:
			return fmt.Errorf("waiting for JOIN response timeout exceeded")
		}
	}
}

const (
	// TagBadgeInfo ...
	TagBadgeInfo string = "badge-info"
	// TagBadges ...
	TagBadges string = "badges"
	// TagColor ...
	TagColor string = "color"
	// TagDisplayName ...
	TagDisplayName string = "display-name"
	// TagEmotes ...
	TagEmotes string = "emotes"
	// TagFlags ...
	TagFlags string = "flags"
	// TagSubscriber 1 if the user has a subscriber badge; otherwise, 0.
	//
	// [deprecated] use badges
	TagSubscriber string = "subscriber"
)

func (c *ChatClient) onError(format string, args ...interface{}) {
	if c.OnError != nil {
		c.OnError(format, args...)
	}
}
func (c *ChatClient) onReceived(rawMsg string) {
	if c.OnReceived != nil {
		c.OnReceived(rawMsg)
	}
}
func (c *ChatClient) listenContext(ctx context.Context) {
	isDigitCode := func(code string) bool {
		for _, c := range code {
			if !unicode.IsDigit(c) {
				return false
			}
		}
		return true
	}

	commandHandlers := map[string]func(*ircv3Message, *Message) error{
		"PRIVMSG": func(im *ircv3Message, msg *Message) error {
			if len(im.Command.Args) < 2 ||
				len(im.Command.Args[0]) < 2 {
				return fmt.Errorf("invalid aruments for %s received", msg.Command)
			}
			if !strings.ContainsRune(msg.Prefix, '!') {
				return fmt.Errorf("could not determine sender for %s", msg.Command)
			}
			msg.Channel = im.Command.Args[0][1:]
			msg.Sender = msg.Prefix[:strings.IndexRune(msg.Prefix, '!')]
			return nil
		},
		"JOIN": func(im *ircv3Message, msg *Message) error {
			if len(im.Command.Args) < 1 {
				return fmt.Errorf("invalid aruments for %s received", msg.Command)
			}
			return nil
		},
		"NOTICE": func(im *ircv3Message, msg *Message) error {
			if len(im.Command.Args) > 0 &&
				len(im.Command.Args[0]) > 1 {
				msg.Channel = im.Command.Args[0][1:]
			}
			return nil
		},
	}

	go func() {
		defer func() {
			c.doneListening <- struct{}{}
		}()
		defer func() {
			c.handlerMtx.Lock()
			defer c.handlerMtx.Unlock()
			for c := range c.messageHandlers {
				close(c)
			}
			for c := range c.messageHandlersInternal {
				close(c)
			}
		}()

		keepGoing := func(scn *bufio.Scanner) bool {
			select {
			case <-ctx.Done():
				return false
			default:
				return scn.Scan()
			}
		}
		br := bytes.NewReader(nil)
		for mt, data, err := c.conn.Read(ctx); mt == websocket.MessageText && err == nil; mt, data, err = c.conn.Read(ctx) {
			br.Reset(data)

			scn := bufio.NewScanner(br)
			for keepGoing(scn) {
				rawMsg := scn.Text()
				ircMsg, err := parseIRCv3(rawMsg)
				if err != nil {
					c.onError("Could not parse message %q. %v", rawMsg, err)
					continue
				}
				if ircMsg.Command.Name == "NOTICE" {
					if len(ircMsg.Command.Args) > 1 {
						if ircMsg.Command.Args[1] == "Improperly formatted auth" {
							c.onError("(%s). Missing OAuth information", rawMsg)
							return
						}
					}
				}

				if ircMsg.Command.Name == "PING" {
					if len(ircMsg.Command.Args) == 0 || // No Args
						len(ircMsg.Command.Args[0]) == 0 || ircMsg.Command.Args[0] == "" {
						c.onError("invalid PING received. (%s) %#v", rawMsg, ircMsg.Command)
						return
					}
					if err := c.send("PONG :" + ircMsg.Command.Args[0] + "\n"); err != nil {
						c.onError("Could not send pong. %v", err)
						return
					}
					continue
				}

				msg := &Message{
					Tags: map[string]string{},
				}
				msg.Prefix = ircMsg.Prefix
				msg.Sender = "[SYSTEM]"
				msg.Command = ircMsg.Command.Name
				msg.Args = ircMsg.Command.Args
				if len(msg.Args) > 0 {
					msg.Trailer = msg.Args[len(msg.Args)-1]
				}
				for _, v := range ircMsg.Tags {
					msg.Tags[v.Key] = v.Value
				}

				isPublicMessage := false
				if h, ok := commandHandlers[ircMsg.Command.Name]; ok {
					if err := h(&ircMsg, msg); err != nil {
						c.onError("invalid command %q. %v", rawMsg, err)
						continue
					}
					if ircMsg.Command.Name == "PRIVMSG" {
						isPublicMessage = true
					}
				} else if ircMsg.Command.Name == "PRIVMSG" {
					isPublicMessage = true
				} else {
					if isDigitCode(msg.Command) {
						if len(ircMsg.Command.Args) > 0 {
							msg.Trailer = ircMsg.Command.Args[len(ircMsg.Command.Args)-1]
							msg.Args = ircMsg.Command.Args
						}
					}
				}

				c.handlerMtx.Lock()
				if isPublicMessage {
					for c := range c.messageHandlersInternal {
						c <- msg
					}
					for c := range c.messageHandlers {
						c <- msg
					}
				} else {
					for c := range c.messageHandlersInternal {
						c <- msg
					}
				}
				c.handlerMtx.Unlock()
				c.onReceived(rawMsg)
			}
		}
	}()
}
