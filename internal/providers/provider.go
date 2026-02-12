package providers

import "context"

// Message represents a chat message to/from the LLM.
// Content can be a string (text-only) or a slice of content parts for multimodal (e.g. text + image_url).
type Message struct {
	Role       string      `json:"role"` // "system" | "user" | "assistant" | "tool"
	Content    interface{} `json:"content"` // string or []ContentPart for vision
	ToolCallID string      `json:"tool_call_id,omitempty"` // set when Role == "tool"
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`  // set on assistant msgs with tool calls
}

// ToolDefinition is a lightweight description of a tool available to the model.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a request from the LLM to invoke a tool.
type ToolCall struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Arguments    map[string]interface{} `json:"arguments"`
	ExtraContent map[string]interface{} `json:"extra_content,omitempty"` // Gemini thought_signature etc.; pass through as received
}

// LLMResponse is a normalized response from a provider.
type LLMResponse struct {
	Content      string     `json:"content"`
	HasToolCalls bool       `json:"hasToolCalls"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
}

// LLMProvider is the interface used by the agent loop to call LLMs.
type LLMProvider interface {
	// Chat sends messages to the model and returns a normalized response.
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string) (LLMResponse, error)

	// GetDefaultModel returns the provider's default model string.
	GetDefaultModel() string
}

// ContentToString extracts a string from Message.Content (string or array of parts).
func ContentToString(c interface{}) string {
	if c == nil {
		return ""
	}
	if s, ok := c.(string); ok {
		return s
	}
	if arr, ok := c.([]interface{}); ok {
		for _, p := range arr {
			if m, ok := p.(map[string]interface{}); ok && m["type"] == "text" {
				if t, ok := m["text"].(string); ok {
					return t
				}
			}
		}
	}
	return ""
}
