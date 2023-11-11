package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OAuth2ValidateResponse struct {
	ClientID  string   `json:"client_id"`
	Login     string   `json:"login"`
	Scopes    []string `json:"scopes"`
	UserID    string   `json:"user_id"`
	ExpiresIn int      `json:"expires_in"`
}

func OAuth2Validate(ctx context.Context, token string) (OAuth2ValidateResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", OAuth2ValidateURL, nil)
	if err != nil {
		return OAuth2ValidateResponse{}, err
	}

	req.Header.Set("Authorization", "OAuth "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return OAuth2ValidateResponse{}, err
	}

	defer resp.Body.Close()

	var data OAuth2ValidateResponse
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return OAuth2ValidateResponse{}, fmt.Errorf("non success response code %q: %d", resp.Status, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return OAuth2ValidateResponse{}, err
	}

	return data, nil
}
