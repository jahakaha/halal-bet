package sofascore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testServer(body string, status int) (*httptest.Server, *Client) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	c := New("test-key")
	c.httpClient = &http.Client{
		Transport: redirectTransport(srv.URL),
	}
	return srv, c
}

type redirectTransport string

func (base redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = string(base)[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

const eventsJSON = `{
  "events": [
    {"id": 456, "homeTeam": {"name": "PSG"}, "awayTeam": {"name": "Arsenal"}}
  ]
}`

const incidentsJSON = `{
  "incidents": [
    {"incidentType": "card",  "cardType": "red"},
    {"incidentType": "goal",  "scoringType": "penalty"},
    {"incidentType": "goal",  "scoringType": "regular", "from": "owngoal"},
    {"incidentType": "goal",  "scoringType": "regular"}
  ]
}`

func TestGetEventsByDate(t *testing.T) {
	srv, c := testServer(eventsJSON, http.StatusOK)
	defer srv.Close()

	events, err := c.GetEventsByDate(context.Background(), time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID != 456 {
		t.Errorf("expected ID 456, got %d", events[0].ID)
	}
	if events[0].HomeTeam.Name != "PSG" {
		t.Errorf("expected PSG, got %q", events[0].HomeTeam.Name)
	}
}

func TestGetIncidents(t *testing.T) {
	srv, c := testServer(incidentsJSON, http.StatusOK)
	defer srv.Close()

	incidents, err := c.GetIncidents(context.Background(), 456)
	if err != nil {
		t.Fatal(err)
	}
	if len(incidents) != 4 {
		t.Fatalf("expected 4 incidents, got %d", len(incidents))
	}
}

func TestParseEvents(t *testing.T) {
	incidents := []Incident{
		{IncidentType: "card", CardType: "red"},
		{IncidentType: "goal", ScoringType: "penalty"},
		{IncidentType: "goal", ScoringType: "regular", From: "owngoal"},
		{IncidentType: "goal", ScoringType: "regular"},
		{IncidentType: "card", CardType: "yellow"},
	}

	red, pen, own := ParseEvents(incidents)
	if !red {
		t.Error("expected hadRedCard=true")
	}
	if !pen {
		t.Error("expected hadPenalty=true")
	}
	if !own {
		t.Error("expected hadOwnGoal=true")
	}
}

func TestParseEvents_Empty(t *testing.T) {
	red, pen, own := ParseEvents(nil)
	if red || pen || own {
		t.Error("expected all false for empty incidents")
	}
}

func TestParseEvents_YellowCardNotRed(t *testing.T) {
	incidents := []Incident{
		{IncidentType: "card", CardType: "yellow"},
	}
	red, _, _ := ParseEvents(incidents)
	if red {
		t.Error("yellow card should not set hadRedCard")
	}
}

func TestParseEvents_YellowRed(t *testing.T) {
	incidents := []Incident{
		{IncidentType: "card", CardType: "yellowRed"},
	}
	red, _, _ := ParseEvents(incidents)
	if !red {
		t.Error("yellowRed card should set hadRedCard")
	}
}

func TestGetEventsByDate_HTTPError(t *testing.T) {
	srv, c := testServer(`{}`, http.StatusTooManyRequests)
	defer srv.Close()

	_, err := c.GetEventsByDate(context.Background(), time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

