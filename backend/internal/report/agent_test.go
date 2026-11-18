package report

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func sampleTools() *Tools {
	return &Tools{
		Data: func(context.Context) (*Dataset, error) { return SampleDataset(SampleNow), nil },
		Now:  func() time.Time { return SampleNow },
		Listings: func(_ context.Context, country string, apps []App) ([]Listing, error) {
			return []Listing{{Name: "Habitual", IsMine: true, Rating: 4.6, RatingCount: 812, ArtworkURL: "https://example.com/icon.png"}}, nil
		},
		Previous: func(context.Context) (*Previous, error) { return nil, nil },
	}
}

// Every MCP tool must run and satisfy its own output schema.
func TestMCPToolsRoundTrip(t *testing.T) {
	ctx := context.Background()
	a, b := mcp.NewInMemoryTransports()
	ss, err := sampleTools().Server().Connect(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "1"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 8 {
		t.Fatalf("want 8 tools, got %d", len(listed.Tools))
	}

	calls := map[string]map[string]any{
		"get_overview":         {"days": 7},
		"get_movers":           {"limit": 3},
		"get_opportunities":    {},
		"get_competitor_moves": {},
		"get_keyword_history":  {"keyword": "streaks", "days": 10},
		"find_keywords":        {"query": "habit"},
		"get_app_listings":     {},
		"get_previous_report":  {},
	}
	for name, args := range calls {
		text, isErr := callTool(ctx, cs, name, mustJSON(args))
		if isErr {
			t.Errorf("%s failed: %s", name, text)
			continue
		}
		if !json.Valid([]byte(text)) {
			t.Errorf("%s returned non-JSON: %.80s", name, text)
		}
	}

	if text, isErr := callTool(ctx, cs, "get_keyword_history", `{"keyword":"no such term"}`); !isErr || !strings.Contains(text, "unknown keyword") {
		t.Errorf("expected a tool error for an unknown keyword, got %q (err=%v)", text, isErr)
	}

	text, _ := callTool(ctx, cs, "get_movers", `{"limit":2}`)
	var got moversOut
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Up) != 2 || got.Up[0].Term != "gratitude journal" && got.Up[0].Term != "mood journal" {
		t.Errorf("unexpected movers: %+v", got.Up)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// fakeAnthropic scripts a two-turn conversation: Claude asks for two tools in
// parallel, then answers with the report JSON. It speaks the streaming (SSE)
// protocol the agent uses. failures lists HTTP statuses to answer with first,
// one per request, before behaving normally.
type fakeAnthropic struct {
	mu       sync.Mutex
	bodies   []map[string]any // conversation requests only (failed attempts excluded)
	header   http.Header
	answer   string
	failures []int
	requests int // every request, failed or not
}

func (f *fakeAnthropic) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)

	f.mu.Lock()
	f.requests++
	if f.requests <= len(f.failures) {
		status := f.failures[f.requests-1]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"code":499,"detail":"Client disconnected","client_error":true}`))
		return
	}
	f.bodies = append(f.bodies, body)
	f.header = r.Header.Clone()
	n := len(f.bodies)
	f.mu.Unlock()

	if body["stream"] != true {
		http.Error(w, "the agent must stream", http.StatusBadRequest)
		return
	}

	var blocks []map[string]any
	stop := "end_turn"
	if n == 1 {
		stop = "tool_use"
		blocks = []map[string]any{
			{"type": "text", "text": "Let me look."},
			{"type": "tool_use", "id": "toolu_1", "name": "get_overview", "input": map[string]any{"days": 7}},
			{"type": "tool_use", "id": "toolu_2", "name": "get_keyword_history", "input": map[string]any{"keyword": "nonexistent"}},
		}
	} else {
		blocks = []map[string]any{{"type": "text", "text": f.answer}}
	}
	writeSSE(w, blocks, stop)
}

// writeSSE streams a message the way the API does: start, each content block
// as start/delta/stop, then the stop reason and usage.
func writeSSE(w http.ResponseWriter, blocks []map[string]any, stop string) {
	w.Header().Set("Content-Type", "text/event-stream")
	emit := func(event string, data map[string]any) {
		b, _ := json.Marshal(data)
		_, _ = w.Write([]byte("event: " + event + "\ndata: " + string(b) + "\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
	usage := map[string]any{"input_tokens": 100, "output_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}
	emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_x", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
		"content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": usage}})
	for i, b := range blocks {
		start := map[string]any{"type": b["type"]}
		var delta map[string]any
		switch b["type"] {
		case "text":
			start["text"] = ""
			delta = map[string]any{"type": "text_delta", "text": b["text"]}
		case "tool_use":
			start["id"], start["name"], start["input"] = b["id"], b["name"], map[string]any{}
			in, _ := json.Marshal(b["input"])
			delta = map[string]any{"type": "input_json_delta", "partial_json": string(in)}
		}
		emit("content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": start})
		emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": delta})
		emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	emit("message_delta", map[string]any{"type": "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]any{"output_tokens": 50}})
	emit("message_stop", map[string]any{"type": "message_stop"})
}

func TestGeneratorEndToEndWithStubbedClaude(t *testing.T) {
	answer, err := json.Marshal(SampleContent())
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeAnthropic{answer: string(answer)}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	g := &Generator{Agent: &Agent{
		Client: anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
		Tools:  sampleTools(),
	}}
	rep, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if rep.Usage.Turns != 2 || rep.Usage.Input != 200 || rep.Usage.Output != 100 {
		t.Errorf("usage = %+v", rep.Usage)
	}
	if !strings.Contains(rep.Email.HTML, "Streaks hit the summit") || !strings.Contains(rep.Email.HTML, "https://example.com/icon.png") {
		t.Error("email missing headline or app icon")
	}

	// Request 1: tools came from MCP, schema and fallbacks are set.
	first := fake.bodies[0]
	if first["model"] != DefaultModel {
		t.Errorf("model = %v", first["model"])
	}
	tools, _ := first["tools"].([]any)
	if len(tools) != 8 {
		t.Fatalf("want 8 tools sent to Claude, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, x := range tools {
		tool := x.(map[string]any)
		names[tool["name"].(string)] = true
		if tool["input_schema"].(map[string]any)["type"] != "object" {
			t.Errorf("tool %v has no object schema", tool["name"])
		}
	}
	if !names["get_overview"] || !names["get_opportunities"] {
		t.Errorf("tool names = %v", names)
	}
	oc := first["output_config"].(map[string]any)
	if oc["format"].(map[string]any)["type"] != "json_schema" || oc["effort"] != "high" {
		t.Errorf("output_config = %v", oc)
	}
	if first["stream"] != true || first["max_tokens"] != float64(maxOutputTokens) {
		t.Errorf("stream=%v max_tokens=%v", first["stream"], first["max_tokens"])
	}
	if first["fallbacks"] != "default" {
		t.Errorf("fallbacks = %v", first["fallbacks"])
	}
	if !strings.Contains(strings.Join(fake.header.Values("Anthropic-Beta"), ","), "server-side-fallback-2026-07-01") {
		t.Errorf("beta header = %v", fake.header.Values("Anthropic-Beta"))
	}

	// Request 2: both tool results came back in ONE user message; the bad call
	// is reported as an error rather than aborting the run.
	msgs := fake.bodies[1]["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	if last["role"] != "user" {
		t.Fatalf("last role = %v", last["role"])
	}
	results := last["content"].([]any)
	if len(results) != 2 {
		t.Fatalf("want 2 tool_result blocks in one message, got %d", len(results))
	}
	ok := results[0].(map[string]any)
	if ok["tool_use_id"] != "toolu_1" || !strings.Contains(mustJSON(ok["content"]), "visibility") {
		t.Errorf("overview result = %v", ok)
	}
	bad := results[1].(map[string]any)
	if bad["tool_use_id"] != "toolu_2" || bad["is_error"] != true {
		t.Errorf("error result = %v", bad)
	}
}

func TestGeneratorRejectsGarbage(t *testing.T) {
	fake := &fakeAnthropic{answer: `{"headline":"x"}`}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	g := &Generator{Agent: &Agent{
		Client: anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
		Tools:  sampleTools(),
	}}
	if _, err := g.Generate(context.Background()); err == nil {
		t.Fatal("a report without an intro or quests must not be sent")
	}
}

// A dropped connection (the API reports 499 "client disconnected") is retried
// and the report still completes.
func TestAgentRetriesDroppedConnections(t *testing.T) {
	answer, _ := json.Marshal(SampleContent())
	fake := &fakeAnthropic{answer: string(answer), failures: []int{499, 529}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	var logs []string
	g := &Generator{Agent: &Agent{
		Client:     anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
		Tools:      sampleTools(),
		RetryDelay: time.Millisecond,
		Log:        func(msg string, _ ...any) { logs = append(logs, msg) },
	}}
	rep, err := g.Generate(context.Background())
	if err != nil {
		t.Fatalf("a transient 499 then 529 should be retried: %v", err)
	}
	if rep.Usage.Turns != 2 {
		t.Errorf("turns = %d", rep.Usage.Turns)
	}
	if fake.requests != 4 { // 2 failed attempts + 2 turns
		t.Errorf("requests = %d, want 4", fake.requests)
	}
	retries := 0
	for _, l := range logs {
		if l == "claude request failed, retrying" {
			retries++
		}
	}
	if retries != 2 {
		t.Errorf("logged %d retries, want 2", retries)
	}
}

// Retries are bounded, and a real client error is not retried at all.
func TestAgentRetryLimits(t *testing.T) {
	cases := []struct {
		name         string
		failures     []int
		wantRequests int
	}{
		{"gives up after three attempts", []int{499, 499, 499, 499}, attemptsPerTurn},
		{"bad request is not retried", []int{400}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &fakeAnthropic{answer: "{}", failures: c.failures}
			srv := httptest.NewServer(fake)
			defer srv.Close()
			g := &Generator{Agent: &Agent{
				Client:     anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
				Tools:      sampleTools(),
				RetryDelay: time.Millisecond,
			}}
			if _, err := g.Generate(context.Background()); err == nil {
				t.Fatal("expected an error")
			}
			if fake.requests != c.wantRequests {
				t.Errorf("requests = %d, want %d", fake.requests, c.wantRequests)
			}
		})
	}
}

// Cancelling the context (for example on shutdown) must stop immediately and
// never be treated as a transient fault to retry.
func TestAgentDoesNotRetryCancellation(t *testing.T) {
	fake := &fakeAnthropic{answer: "{}", failures: []int{499, 499, 499}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := &Generator{Agent: &Agent{
		Client:     anthropic.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0)),
		Tools:      sampleTools(),
		RetryDelay: time.Millisecond,
	}}
	if _, err := g.Generate(ctx); err == nil {
		t.Fatal("expected an error")
	}
	if fake.requests > 1 {
		t.Errorf("a cancelled run made %d requests", fake.requests)
	}
}
