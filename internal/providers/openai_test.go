package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIFunctionCallParsing(t *testing.T) {
	// Build a fake server that returns a tool_calls style response
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
		  "choices": [
		    {
		      "message": {
		        "role": "assistant",
		        "content": "",
		        "tool_calls": [
		          {
		            "id": "call_001",
		            "type": "function",
		            "function": {
		              "name": "message",
		              "arguments": "{\"content\": \"Hello from function\"}"
		            }
		          }
		        ]
		      }
		    }
		  ]
		}`))
	}))
	defer h.Close()

	p := NewOpenAIProvider("test-key", h.URL)
	p.Client = &http.Client{Timeout: 5 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgs := []Message{{Role: "user", Content: "trigger"}}
	resp, err := p.Chat(ctx, msgs, nil, "model-x")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.HasToolCalls || len(resp.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got: has=%v len=%d", resp.HasToolCalls, len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "message" {
		t.Fatalf("expected tool name 'message', got '%s'", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["content"] != "Hello from function" {
		t.Fatalf("unexpected argument content: %v", resp.ToolCalls[0].Arguments)
	}
}

func TestOpenAIThoughtSignaturePreservation(t *testing.T) {
	// Gemini returns extra_content with thought_signature; we must preserve and echo it back
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body to verify we're sending thought_signature back
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		bodyStr := string(body)

		// First request: no tool calls in history. Return tool call with thought_signature.
		if !strings.Contains(bodyStr, "tool_calls") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			w.Write([]byte(`{
			  "choices": [{
			    "message": {
			      "role": "assistant",
			      "content": "",
			      "tool_calls": [{
			        "id": "call_gemini_1",
			        "type": "function",
			        "extra_content": {"google": {"thought_signature": "sig-abc123"}},
			        "function": {"name": "web", "arguments": "{\"query\": \"test\"}"}
			      }]
			    }
			  }]
			}`))
			return
		}

		// Second request: must include thought_signature in assistant message's tool_calls
		if !strings.Contains(bodyStr, `"thought_signature":"sig-abc123"`) {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":{"message":"Function call missing thought_signature"}}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Done."}}]}`))
	}))
	defer h.Close()

	p := NewOpenAIProvider("test-key", h.URL)
	p.Client = &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// First call: get tool call with thought_signature
	msgs := []Message{{Role: "user", Content: "search for x"}}
	resp, err := p.Chat(ctx, msgs, []ToolDefinition{{Name: "web", Description: "Search"}}, "gemini")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if !resp.HasToolCalls || len(resp.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got has=%v len=%d", resp.HasToolCalls, len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ExtraContent == nil {
		t.Fatal("expected ExtraContent to be preserved from API response")
	}
	google, ok := resp.ToolCalls[0].ExtraContent["google"].(map[string]interface{})
	if !ok || google["thought_signature"] != "sig-abc123" {
		t.Fatalf("expected thought_signature, got %v", resp.ToolCalls[0].ExtraContent)
	}

	// Second call: send history with tool call + tool result; must include thought_signature
	messages := []Message{
		{Role: "user", Content: "search for x"},
		{Role: "assistant", Content: "", ToolCalls: resp.ToolCalls},
		{Role: "tool", Content: "result", ToolCallID: resp.ToolCalls[0].ID},
	}
	resp2, err := p.Chat(ctx, messages, []ToolDefinition{{Name: "web", Description: "Search"}}, "gemini")
	if err != nil {
		t.Fatalf("second call (with history): %v", err)
	}
	if resp2.Content != "Done." {
		t.Fatalf("expected final content, got %q", resp2.Content)
	}
}
