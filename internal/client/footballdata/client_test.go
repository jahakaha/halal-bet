package footballdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

const sampleMatchesJSON = `{
  "matches": [
    {
      "id": 123,
      "utcDate": "2026-06-15T17:00:00Z",
      "status": "TIMED",
      "matchday": 1,
      "stage": "GROUP_STAGE",
      "group": "GROUP_A",
      "homeTeam": {"name": "Brazil"},
      "awayTeam": {"name": "Mexico"},
      "score": {"fullTime": {"home": null, "away": null}}
    }
  ]
}`

func TestGetWC2026Matches(t *testing.T) {
	srv := testServer(sampleMatchesJSON, http.StatusOK)
	defer srv.Close()

	c := New("test-token")
	c.httpClient = srv.Client()
	// override baseURL by using a custom request — we need to intercept the URL
	// Instead, use a transport that redirects to the test server
	c.httpClient.Transport = redirectTransport(srv.URL)

	matches, err := c.GetWC2026Matches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.ID != 123 {
		t.Errorf("expected ID 123, got %d", m.ID)
	}
	if m.HomeTeam.Name != "Brazil" {
		t.Errorf("expected Brazil, got %q", m.HomeTeam.Name)
	}
	if m.Status != "TIMED" {
		t.Errorf("expected TIMED, got %q", m.Status)
	}
	if m.Group == nil || *m.Group != "GROUP_A" {
		t.Errorf("expected GROUP_A, got %v", m.Group)
	}
}

func TestGetWC2026Matches_HTTPError(t *testing.T) {
	srv := testServer(`{"error":"unauthorized"}`, http.StatusUnauthorized)
	defer srv.Close()

	c := New("bad-token")
	c.httpClient = srv.Client()
	c.httpClient.Transport = redirectTransport(srv.URL)

	_, err := c.GetWC2026Matches(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGetWC2026Matches_EmptyList(t *testing.T) {
	srv := testServer(`{"matches":[]}`, http.StatusOK)
	defer srv.Close()

	c := New("token")
	c.httpClient = srv.Client()
	c.httpClient.Transport = redirectTransport(srv.URL)

	matches, err := c.GetWC2026Matches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

// redirectTransport rewrites all requests to the given base URL.
type redirectTransport string

func (base redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Host = req.URL.Host // keep path/query
	// point scheme+host to test server
	req2.URL.Scheme = "http"
	req2.URL.Host = string(base)[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}
