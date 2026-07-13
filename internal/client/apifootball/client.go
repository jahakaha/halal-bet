package apifootball

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://v3.football.api-sports.io"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type Fixture struct {
	ID       int64
	HomeTeam string
	AwayTeam string
}

type fixtureItem struct {
	Fixture struct {
		ID     int64 `json:"id"`
		Status struct {
			Short string `json:"short"`
		} `json:"status"`
	} `json:"fixture"`
	League struct {
		Name string `json:"name"`
	} `json:"league"`
	Teams struct {
		Home struct{ Name string `json:"name"` } `json:"home"`
		Away struct{ Name string `json:"name"` } `json:"away"`
	} `json:"teams"`
}

type fixtureResponse struct {
	Response []fixtureItem `json:"response"`
}

type Event struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
	Time   struct {
		Elapsed int  `json:"elapsed"`
		Extra   *int `json:"extra"`
	} `json:"time"`
}

type eventsResponse struct {
	Response []Event `json:"response"`
}

// GetFixturesByDate returns all fixtures for the given date (YYYY-MM-DD),
// filtered to World Cup matches only. Free plan supports date-based queries.
func (c *Client) GetFixturesByDate(ctx context.Context, date time.Time) ([]Fixture, error) {
	url := fmt.Sprintf("%s/fixtures?date=%s", baseURL, date.Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apifootball: get-fixtures status %d", resp.StatusCode)
	}

	var result fixtureResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var fixtures []Fixture
	for _, item := range result.Response {
		if isWorldCup(item.League.Name) {
			fixtures = append(fixtures, Fixture{
				ID:       item.Fixture.ID,
				HomeTeam: item.Teams.Home.Name,
				AwayTeam: item.Teams.Away.Name,
			})
		}
	}
	return fixtures, nil
}

// GetFixtureEvents returns all events (goals, cards) for a fixture.
func (c *Client) GetFixtureEvents(ctx context.Context, fixtureID int64) ([]Event, error) {
	url := fmt.Sprintf("%s/fixtures/events?fixture=%d", baseURL, fixtureID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apifootball: get-events status %d for fixture %d", resp.StatusCode, fixtureID)
	}

	var result eventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("x-apisports-key", c.apiKey)
}

func isWorldCup(leagueName string) bool {
	name := strings.ToLower(leagueName)
	return strings.Contains(name, "world cup")
}

// ParseEvents extracts red card, penalty, own goal flags from fixture events.
// Penalty shootout kicks (elapsed > 120) are excluded — only in-game penalties count.
func ParseEvents(events []Event) (hadRedCard, hadPenalty, hadOwnGoal bool) {
	for _, e := range events {
		switch e.Type {
		case "Card":
			if e.Detail == "Red Card" || e.Detail == "Yellow Red Card" {
				hadRedCard = true
			}
		case "Var":
			if e.Detail == "Red Card" {
				hadRedCard = true
			}
			if e.Detail == "Penalty Confirmed" && e.Time.Elapsed <= 120 {
				hadPenalty = true
			}
		case "Goal":
			if e.Detail == "Penalty" && e.Time.Elapsed <= 120 {
				hadPenalty = true
			}
			if e.Detail == "Own Goal" {
				hadOwnGoal = true
			}
		case "Miss":
			if e.Detail == "Missed Penalty" && e.Time.Elapsed <= 120 {
				hadPenalty = true
			}
		}
	}
	return
}
