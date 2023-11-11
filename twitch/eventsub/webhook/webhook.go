package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kirides/twitch-integration/twitch"
	"github.com/kirides/twitch-integration/twitch/eventsub"
)

type WebhookManager struct {
	clientID             string
	clientSecret         string
	appToken             string
	appTokenExpiresAt    time.Time
	http                 *http.Client
	pendingVerifications []string
	mtxVerifications     *sync.Mutex
	handler              *eventsub.Handler
	subscriptions        *sync.Map
}

func New(clientID, clientSecret string, handler *eventsub.Handler) *WebhookManager {
	return &WebhookManager{
		clientID:             clientID,
		clientSecret:         clientSecret,
		appTokenExpiresAt:    time.Now(),
		http:                 http.DefaultClient,
		pendingVerifications: make([]string, 0),
		mtxVerifications:     &sync.Mutex{},
		handler:              handler,
		subscriptions:        &sync.Map{},
	}
}

func GenerateSecret() (string, error) {
	var data [16]byte
	_, err := io.ReadFull(rand.Reader, data[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

// Subscriptions returns all subscriptions that were handled atleast once on this manager instance
func (m *WebhookManager) Subscriptions() []eventsub.Subscription {
	var result []eventsub.Subscription
	m.subscriptions.Range(func(_, value interface{}) bool {
		result = append(result, value.(eventsub.Subscription))
		return true
	})

	return result
}

// kirides

type User struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	// Type            string    `json:"type"`
	// BroadcasterType string    `json:"broadcaster_type"`
	Description     string `json:"description"`
	ProfileImageURL string `json:"profile_image_url"`
	OfflineImageURL string `json:"offline_image_url"`
	ViewCount       int    `json:"view_count"`
	// Email           string    `json:"email"`
	// CreatedAt       time.Time `json:"created_at"`
}

// AllSubscriptions calls out to twitch to receive a list of all subscriptions
func (m *WebhookManager) GetUser(username string) (User, error) {
	var result User
	values := url.Values{}
	values.Set("login", username)
	req, err := m.NewAuthdRequest(http.MethodGet, twitch.QueryUsersURL(values), nil)
	if err != nil {
		return result, err
	}

	resp, err := m.http.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("response error: %s (%d)", resp.Status, resp.StatusCode)
	}

	type ResType struct {
		Data []User `json:"data"`
	}
	var resVal ResType
	if err := json.NewDecoder(resp.Body).Decode(&resVal); err != nil {
		return result, err
	}
	for _, v := range resVal.Data {
		return v, nil
	}
	return result, nil
}

func (m *WebhookManager) NewAuthdRequest(method, uri string, body io.Reader) (*http.Request, error) {
	if err := m.UpdateAccessToken(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Client-ID", m.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.appToken))
	return req, nil
}

// AllSubscriptions calls out to twitch to receive a list of all subscriptions
func (m *WebhookManager) AllSubscriptions() ([]eventsub.Subscription, error) {
	var result []eventsub.Subscription
	req, err := m.NewAuthdRequest(http.MethodGet, twitch.EventSubSubscriptionsURL, nil)
	if err != nil {
		return result, err
	}

	resp, err := m.http.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("response error: %s (%d)", resp.Status, resp.StatusCode)
	}

	type ResType struct {
		Data         []eventsub.Subscription `json:"data"`
		Total        int                     `json:"total"`
		TotalCost    int                     `json:"total_cost"`
		MaxTotalCost int                     `json:"max_total_cost"`
		// Pagination   struct{}                `json:"pagination"`
	}
	var resVal ResType
	if err := json.NewDecoder(resp.Body).Decode(&resVal); err != nil {
		return result, err
	}
	if resVal.Data != nil {
		result = append(result, resVal.Data...)
	}
	return result, nil
}
func (m *WebhookManager) UpdateAccessToken() error {
	if time.Now().Add(time.Minute).Before(m.appTokenExpiresAt) && m.appToken != "" {
		return nil
	}
	values := url.Values{}
	values.Set("client_id", m.clientID)
	values.Set("client_secret", m.clientSecret)
	values.Set("grant_type", "client_credentials")
	values.Set("scope", "channel:read:redemptions user:read:follows")

	req, err := http.NewRequest(http.MethodPost, twitch.QueryOAuth2TokenURL(values), nil)
	if err != nil {
		return err
	}

	now := time.Now()
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("response error: %s (%d)", resp.Status, resp.StatusCode)
	}

	type tokenResponse struct {
		AccessToken string   `json:"access_token"`
		ExpiresIn   int      `json:"expires_in"`
		Scope       []string `json:"scope"`
		TokenType   string   `json:"token_type"`
	}
	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil
	}
	m.appToken = tokenResp.AccessToken
	m.appTokenExpiresAt = now.Add(time.Second * time.Duration(tokenResp.ExpiresIn))
	return nil
}

func (m *WebhookManager) DeleteSubscription(id string) error {
	values := url.Values{}
	values.Set("id", id)
	req, err := m.NewAuthdRequest(http.MethodDelete, twitch.QuerySubscriptionsURL(values), nil)
	if err != nil {
		return err
	}

	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("response error: %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

func (m *WebhookManager) Subscribe(info SubscriptionInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	body := bytes.NewReader(data)
	req, err := m.NewAuthdRequest(http.MethodPost, twitch.EventSubSubscriptionsURL, body)
	if err != nil {
		return err
	}
	defer req.Body.Close()
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("response error: %s (%d)", resp.Status, resp.StatusCode)
	}

	respRdr := io.LimitReader(resp.Body, 4*1024*1024)
	respBody, err := io.ReadAll(respRdr)
	if err != nil {
		return err
	}
	var respData SubscriptionResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		return err
	}

	if len(respData.Data) == 0 || len(respData.Data) > 1 {
		return fmt.Errorf("too much response data")
	}

	if respData.Data[0].Status == "webhook_callback_verification_pending" {
		m.mtxVerifications.Lock()
		defer m.mtxVerifications.Unlock()
		m.pendingVerifications = append(m.pendingVerifications, info.Transport.Secret)
	}

	return nil
}

func getRawRequest(r *http.Request, data []byte) string {
	b := strings.Builder{}
	b.WriteString(r.Method)
	b.WriteString(" ")
	b.WriteString(r.URL.Path)
	b.WriteString("\n")
	b.WriteString("Query:\n")
	for k, v := range r.URL.Query() {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(v, ", "))
		b.WriteString("\n")
	}
	b.WriteString("Headers:\n")
	for k, v := range r.Header {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(strings.Join(v, ", "))
		b.WriteString("\n")
	}
	b.WriteString("Body:\n")
	b.Write(data)

	return b.String()
}

func (m *WebhookManager) CallbackHandler(log func(string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handled := m.verifyOrRevokeWebhook(w, r); handled {
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		var subHandler eventsub.AnonymousNotification
		if err := json.Unmarshal(data, &subHandler); err != nil {
			log("could not unmarshal incoming subscription")
			log(getRawRequest(r, data))
			http.Error(w, "not a twitch webhook subscription", http.StatusBadRequest)
			return
		} else {
			m.subscriptions.Store(subHandler.Subscription.ID, subHandler.Subscription)
			if m.handler.OnAny != nil {
				m.handler.OnAny(subHandler)
			}
		}
		subType := r.Header.Get("Twitch-Eventsub-Subscription-Type")
		m.handler.Delegate(subType, data)
	})
}

func (m *WebhookManager) verifyOrRevokeWebhook(w http.ResponseWriter, r *http.Request) bool {
	verifyData, err := getVerificationData(r)
	if err != nil {
		return false
	}

	respRdr := io.LimitReader(r.Body, 4*1024*1024)
	respBody, err := io.ReadAll(respRdr)
	if err != nil {
		http.Error(w, "invalid request", http.StatusForbidden)
		return true
	}
	data := []byte(verifyData.Message.ID + verifyData.Message.Timestamp)
	data = append(data, respBody...)
	sig, err := hex.DecodeString(verifyData.Message.Signature[7:])
	if err != nil {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return true
	}

	if verifyData.Message.Type == "revocation" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	verificationRequest := struct {
		Challenge    string `json:"challenge"`
		Subscription struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			Type      string `json:"type"`
			Version   string `json:"version"`
			Cost      int    `json:"cost"`
			Condition struct {
				BroadcasterUserID string `json:"broadcaster_user_id"`
			} `json:"condition"`
			Transport struct {
				Method   string `json:"method"`
				Callback string `json:"callback"`
			} `json:"transport"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"subscription"`
	}{}
	if err := json.Unmarshal(respBody, &verificationRequest); err != nil {
		http.Error(w, "invalid body", http.StatusForbidden)
		return true
	}

	m.mtxVerifications.Lock()
	defer m.mtxVerifications.Unlock()
	for i, secret := range m.pendingVerifications {
		if validMAC(data, sig, []byte(secret)) {
			m.pendingVerifications = append(m.pendingVerifications[:i], m.pendingVerifications[i+1:]...)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(verificationRequest.Challenge))
			return true
		}
	}
	http.Error(w, "not a pending request", http.StatusForbidden)
	return true
}

func validMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	return hmac.Equal(messageMAC, expectedMAC)
}

func getVerificationData(r *http.Request) (*callbackVerificationData, error) {
	messageID := r.Header.Get("Twitch-Eventsub-Message-Id")
	messageRetry, _ := strconv.Atoi(r.Header.Get("Twitch-Eventsub-Message-Retry"))
	messageType := r.Header.Get("Twitch-Eventsub-Message-Type")
	messageSignature := r.Header.Get("Twitch-Eventsub-Message-Signature")
	messageTimestamp := r.Header.Get("Twitch-Eventsub-Message-Timestamp")
	subscriptionType := r.Header.Get("Twitch-Eventsub-Subscription-Type")
	subscriptionVersion := r.Header.Get("Twitch-Eventsub-Subscription-Version")

	if messageType != "webhook_callback_verification" && messageType != "revocation" {
		return nil, fmt.Errorf("not a verification request")
	}

	if len(messageSignature) < 7 {
		return nil, fmt.Errorf("invalid signature header")
	}

	return &callbackVerificationData{
		Message: struct {
			ID        string
			Retry     int
			Type      string
			Signature string
			Timestamp string
		}{
			ID:        messageID,
			Retry:     messageRetry,
			Type:      messageType,
			Signature: messageSignature,
			Timestamp: messageTimestamp,
		},
		Subscription: struct {
			Type    string
			Version string
		}{
			Type:    subscriptionType,
			Version: subscriptionVersion,
		},
	}, nil
}
