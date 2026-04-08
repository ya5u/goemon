package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

type WebFetch struct {
	client *http.Client
}

func NewWebFetch() *WebFetch {
	return &WebFetch{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *WebFetch) Name() string { return "web_fetch" }
func (w *WebFetch) Description() string {
	return "Fetch a web page via HTTP GET. HTML tags are stripped."
}

func (w *WebFetch) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to fetch",
			},
		},
		"required": []string{"url"},
	}
}

func (w *WebFetch) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("web_fetch", "url", params.URL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "GoEmon/1.0")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	text := stripHTML(string(body))
	return fmt.Sprintf("Status: %d\n\n%s", resp.StatusCode, text), nil
}

var (
	scriptRe  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlTagRe = regexp.MustCompile(`<[^>]*>`)
)

func stripHTML(s string) string {
	s = scriptRe.ReplaceAllString(s, "")
	s = styleRe.ReplaceAllString(s, "")
	return htmlTagRe.ReplaceAllString(s, "")
}
