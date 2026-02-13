package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// WebTool supports fetch operations.
// Args: {"url": "https://...", "headers": {"Header-Name": "value"} (optional)}

type WebTool struct{}

func NewWebTool() *WebTool { return &WebTool{} }

func (t *WebTool) Name() string        { return "web" }
func (t *WebTool) Description() string { return "Fetch web content from a URL" }

func (t *WebTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to fetch (must be http or https)",
			},
			"headers": map[string]interface{}{
				"type": "object",
				"description": "Optional HTTP headers to send with the request",
				"additionalProperties": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"url"},
	}
}

func (t *WebTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	u, ok := args["url"].(string)
	if !ok || u == "" {
		return "", fmt.Errorf("web: 'url' argument required")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	if headersRaw, ok := args["headers"]; ok {
		headers, ok := headersRaw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("web: 'headers' must be an object")
		}
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
