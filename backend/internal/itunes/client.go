package itunes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://itunes.apple.com"

// App is the subset of the API's software result we care about.
type App struct {
	TrackID    int64  `json:"trackId"`
	TrackName  string `json:"trackName"`
	BundleID   string `json:"bundleId"`
	SellerName string `json:"sellerName"`
	Artwork    string `json:"artworkUrl100"`
}

type response struct {
	ResultCount int   `json:"resultCount"`
	Results     []App `json:"results"`
}

// Client talks to the API through a shared Limiter, retrying transient errors.
type Client struct {
	BaseURL     string
	HTTP        *http.Client
	Limiter     *Limiter
	MaxAttempts int
	// Sleep is overridable so tests don't actually wait.
	Sleep func(ctx context.Context, d time.Duration) error
}

// NewClient builds a client with sensible defaults.
func NewClient(limiter *Limiter) *Client {
	return &Client{
		BaseURL:     DefaultBaseURL,
		HTTP:        &http.Client{Timeout: 20 * time.Second},
		Limiter:     limiter,
		MaxAttempts: 4,
		Sleep:       sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Search returns up to 200 apps for a term in a storefront, in API order.
func (c *Client) Search(ctx context.Context, term, country string) ([]App, error) {
	q := url.Values{}
	q.Set("term", term)
	q.Set("country", country)
	q.Set("media", "software")
	q.Set("entity", "software")
	q.Set("limit", "200")
	return c.get(ctx, c.BaseURL+"/search?"+q.Encode())
}

// Lookup resolves app metadata for one or more track IDs.
func (c *Client) Lookup(ctx context.Context, country string, ids ...int64) ([]App, error) {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	q := url.Values{}
	q.Set("id", strings.Join(parts, ","))
	q.Set("country", country)
	return c.get(ctx, c.BaseURL+"/lookup?"+q.Encode())
}

// RankOf returns the 1-based position of trackID in results, or 0 if absent.
func RankOf(results []App, trackID int64) int {
	for i, a := range results {
		if a.TrackID == trackID {
			return i + 1
		}
	}
	return 0
}

func (c *Client) get(ctx context.Context, u string) ([]App, error) {
	var lastErr error
	for attempt := 0; attempt < c.MaxAttempts; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "aso-tracker/0.1")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			if serr := c.Sleep(ctx, backoff(attempt, "")); serr != nil {
				return nil, serr
			}
			continue
		}

		body, rerr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		retryAfter := resp.Header.Get("Retry-After")

		switch {
		case resp.StatusCode == http.StatusOK:
			if rerr != nil {
				lastErr = rerr
				break
			}
			var r response
			if err := json.Unmarshal(body, &r); err != nil {
				return nil, fmt.Errorf("decode response: %w", err)
			}
			return r.Results, nil

		case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
			c.Limiter.SlowDown()
			lastErr = fmt.Errorf("rate limited: HTTP %d", resp.StatusCode)

		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)

		default:
			return nil, fmt.Errorf("unexpected HTTP %d", resp.StatusCode)
		}

		if attempt < c.MaxAttempts-1 {
			if serr := c.Sleep(ctx, backoff(attempt, retryAfter)); serr != nil {
				return nil, serr
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, fmt.Errorf("after %d attempts: %w", c.MaxAttempts, lastErr)
}

// backoff honours Retry-After (seconds) when present, else 2s * 2^attempt plus jitter.
func backoff(attempt int, retryAfter string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	d := 2 * time.Second << attempt
	return d + time.Duration(rand.Int63n(int64(time.Second)))
}
