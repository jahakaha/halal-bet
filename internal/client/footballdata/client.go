package footballdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://api.football-data.org/v4"

type Client struct {
	token      string
	httpClient *http.Client
}

func New(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type Match struct {
	ID       int64   `json:"id"`
	UTCDate  string  `json:"utcDate"`
	Status   string  `json:"status"`
	Matchday *int    `json:"matchday"`
	Stage    string  `json:"stage"`
	Group    *string `json:"group"`
	HomeTeam struct {
		Name string `json:"name"`
	} `json:"homeTeam"`
	AwayTeam struct {
		Name string `json:"name"`
	} `json:"awayTeam"`
	Score struct {
		FullTime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"fullTime"`
	} `json:"score"`
}

type matchesResponse struct {
	Matches []Match `json:"matches"`
}

func (c *Client) getMatches(ctx context.Context, url string) ([]Match, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("football-data.org: status %d", resp.StatusCode)
	}

	var result matchesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Matches, nil
}

func (c *Client) GetWC2026Matches(ctx context.Context) ([]Match, error) {
	return c.getMatches(ctx, baseURL+"/competitions/WC/matches?season=2026")
}
