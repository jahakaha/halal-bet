package sofascore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://sofascore.p.rapidapi.com"

// WC2026 tournament and season IDs on Sofascore.
const (
	WC2026TournamentID = 16
	WC2026SeasonID     = 58210
)

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

type Event struct {
	ID         int64       `json:"id"`
	HomeTeam   Team        `json:"homeTeam"`
	AwayTeam   Team        `json:"awayTeam"`
	Tournament Tournament  `json:"tournament"`
	Status     EventStatus `json:"status"`
	HomeScore  *EventScore `json:"homeScore"`
	AwayScore  *EventScore `json:"awayScore"`
}

type Team struct {
	Name string `json:"name"`
}

type Tournament struct {
	Name             string           `json:"name"`
	UniqueTournament UniqueTournament `json:"uniqueTournament"`
}

type UniqueTournament struct {
	Name string `json:"name"`
}

type EventStatus struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type EventScore struct {
	Current *int `json:"current"`
	Display *int `json:"display"`
}

type eventsResponse struct {
	Events []Event `json:"events"`
}

type Incident struct {
	IncidentType string `json:"incidentType"`
	CardType     string `json:"cardType"`
	ScoringType  string `json:"scoringType"`
	From         string `json:"from"`
}

type incidentsResponse struct {
	Incidents []Incident `json:"incidents"`
}

// GetWC2026Events returns WC2026 matches from Sofascore for the given page.
func (c *Client) GetWC2026Events(ctx context.Context, pageIndex int) ([]Event, error) {
	url := fmt.Sprintf("%s/tournaments/get-matches?tournamentId=%d&seasonId=%d&pageIndex=%d",
		baseURL, WC2026TournamentID, WC2026SeasonID, pageIndex)

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
		return nil, fmt.Errorf("sofascore: get-matches status %d", resp.StatusCode)
	}

	var result eventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Events, nil
}

func (c *Client) GetIncidents(ctx context.Context, eventID int64) ([]Incident, error) {
	url := fmt.Sprintf("%s/matches/get-incidents?matchId=%d", baseURL, eventID)

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
		return nil, fmt.Errorf("sofascore: get-incidents status %d for event %d", resp.StatusCode, eventID)
	}

	var result incidentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Incidents, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("x-rapidapi-key", c.apiKey)
	req.Header.Set("x-rapidapi-host", "sofascore.p.rapidapi.com")
}

// IsWC2026 returns true if the event belongs to the FIFA World Cup 2026.
func IsWC2026(e Event) bool {
	name := strings.ToLower(e.Tournament.UniqueTournament.Name)
	return strings.Contains(name, "world cup")
}

// ParseEvents extracts event flags from a list of incidents.
func ParseEvents(incidents []Incident) (hadRedCard, hadPenalty, hadOwnGoal bool) {
	for _, inc := range incidents {
		switch inc.IncidentType {
		case "card":
			if inc.CardType == "red" || inc.CardType == "yellowRed" {
				hadRedCard = true
			}
		case "goal":
			if inc.ScoringType == "penalty" {
				hadPenalty = true
			}
			if inc.From == "owngoal" {
				hadOwnGoal = true
			}
		}
	}
	return
}
