package itunes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(url string) *Client {
	c := NewClient(NewLimiter(time.Millisecond))
	c.BaseURL = url
	c.Sleep = func(ctx context.Context, d time.Duration) error { return nil }
	return c
}

func TestSearchParsesAndRanks(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"resultCount":3,"results":[
			{"trackId":11,"trackName":"A"},
			{"trackId":22,"trackName":"B"},
			{"trackId":33,"trackName":"C"}]}`))
	}))
	defer srv.Close()

	apps, err := testClient(srv.URL).Search(context.Background(), "gym log", "us")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 3 {
		t.Fatalf("got %d apps", len(apps))
	}
	if got := RankOf(apps, 22); got != 2 {
		t.Errorf("rank of 22 = %d, want 2", got)
	}
	if got := RankOf(apps, 999); got != 0 {
		t.Errorf("rank of missing = %d, want 0", got)
	}
	for _, want := range []string{"term=gym+log", "country=us", "media=software", "entity=software", "limit=200"} {
		if !contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestRetriesRateLimitAndSlowsDown(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"resultCount":1,"results":[{"trackId":1}]}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	before := c.Limiter.Interval()
	apps, err := c.Search(context.Background(), "x", "us")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || calls != 3 {
		t.Fatalf("apps=%d calls=%d", len(apps), calls)
	}
	if c.Limiter.Interval() <= before {
		t.Errorf("limiter did not slow down: %v -> %v", before, c.Limiter.Interval())
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	if _, err := c.Search(context.Background(), "x", "us"); err == nil {
		t.Fatal("expected error")
	}
	if int(calls) != c.MaxAttempts {
		t.Errorf("calls=%d want %d", calls, c.MaxAttempts)
	}
}

func TestClientErrorIsNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if _, err := testClient(srv.URL).Search(context.Background(), "x", "us"); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("calls=%d want 1", calls)
	}
}

func TestLookupJoinsIDs(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("id")
		w.Write([]byte(`{"resultCount":0,"results":[]}`))
	}))
	defer srv.Close()

	if _, err := testClient(srv.URL).Lookup(context.Background(), "us", 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "1,2,3" {
		t.Errorf("id=%q", gotQuery)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
