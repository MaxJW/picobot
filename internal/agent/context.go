package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/local/picobot/internal/agent/memory"
	"github.com/local/picobot/internal/agent/skills"
	"github.com/local/picobot/internal/providers"
)

// ContextBuilder builds messages for the LLM from session history and current message.
type ContextBuilder struct {
	workspace    string
	ranker       memory.Ranker
	topK         int
	skillsLoader *skills.Loader
}

func NewContextBuilder(workspace string, r memory.Ranker, topK int) *ContextBuilder {
	return &ContextBuilder{
		workspace:    workspace,
		ranker:       r,
		topK:         topK,
		skillsLoader: skills.NewLoader(workspace),
	}
}

// parseHistoryItem extracts role and content from a history item in "role: content" format.
// Returns ("user", content) for "user: hello", ("assistant", content) for "assistant: hi", etc.
// Defaults to "user" if the role is unrecognized.
func parseHistoryItem(h string) (role, content string) {
	const sep = ": "
	idx := strings.Index(h, sep)
	if idx < 0 {
		return "user", h
	}
	role = strings.TrimSpace(strings.ToLower(h[:idx]))
	content = strings.TrimSpace(h[idx+len(sep):])
	switch role {
	case "assistant":
		return "assistant", content
	case "system":
		return "system", content
	case "tool":
		return "user", content
	default:
		return "user", content
	}
}

func (cb *ContextBuilder) BuildMessages(history []string, currentMessage string, media []string, channel, chatID string, memoryContext string, memories []memory.MemoryItem) []providers.Message {
	msgs := make([]providers.Message, 0, len(history)+8)
	// system prompt
	msgs = append(msgs, providers.Message{Role: "system", Content: "You are Picobot, a helpful assistant."})

	// Load workspace bootstrap files (SOUL.md, AGENTS.md, USER.md, TOOLS.md)
	// These define the agent's personality, instructions, and available tools documentation.
	bootstrapFiles := []string{"SOUL.md", "AGENTS.md", "USER.md", "TOOLS.md"}
	for _, name := range bootstrapFiles {
		p := filepath.Join(cb.workspace, name)
		data, err := os.ReadFile(p)
		if err != nil {
			continue // file may not exist yet, skip silently
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			msgs = append(msgs, providers.Message{Role: "system", Content: fmt.Sprintf("## %s\n\n%s", name, content)})
		}
	}

	// instruction for memory tool usage
	msgs = append(msgs, providers.Message{Role: "system", Content: "If you decide something should be remembered, call the tool 'write_memory' with JSON arguments: {\"target\": \"today\"|\"long\", \"content\": \"...\", \"append\": true|false}. Use a tool call rather than plain chat text when writing memory."})

	// after finishing a task with tools, give a normal response so the user knows what you did
	msgs = append(msgs, providers.Message{Role: "system", Content: "When you finish a task and the last step was using a tool, still give a normal conversational response so the user knows what you did. Never leave the user with only raw tool output. Never promise to do something without actually doing it—call the tools immediately."})

	// Load and include skills context
	loadedSkills, err := cb.skillsLoader.LoadAll()
	if err != nil {
		log.Printf("error loading skills: %v", err)
	}
	if len(loadedSkills) > 0 {
		var sb strings.Builder
		sb.WriteString("Available Skills:\n")
		for _, skill := range loadedSkills {
			sb.WriteString(fmt.Sprintf("\n## %s\n%s\n\n%s\n", skill.Name, skill.Description, skill.Content))
		}
		msgs = append(msgs, providers.Message{Role: "system", Content: sb.String()})
	}

	// include file-based memory context (long-term + today's notes) if present
	if memoryContext != "" {
		msgs = append(msgs, providers.Message{Role: "system", Content: "Memory:\n" + memoryContext})
	}

	// select top-K memories using ranker if available
	selected := memories
	if cb.ranker != nil && len(memories) > 0 {
		selected = cb.ranker.Rank(currentMessage, memories, cb.topK)
	}
	if len(selected) > 0 {
		var sb strings.Builder
		sb.WriteString("Relevant memories:\n")
		for _, m := range selected {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", m.Text, m.Kind))
		}
		msgs = append(msgs, providers.Message{Role: "system", Content: sb.String()})
	}

	// replay history
	for _, h := range history {
		if len(h) == 0 {
			continue
		}
		role, content := parseHistoryItem(h)
		msgs = append(msgs, providers.Message{Role: role, Content: content})
	}

	// current user message (with optional images for vision models)
	userContent := buildUserContent(currentMessage, media)
	msgs = append(msgs, providers.Message{Role: "user", Content: userContent})
	return msgs
}

// buildUserContent returns a string or a content array for multimodal (text + images).
func buildUserContent(text string, media []string) interface{} {
	if len(media) == 0 {
		return text
	}
	if strings.TrimSpace(text) == "" {
		text = "[Image attached]"
	}
	parts := []interface{}{
		map[string]interface{}{"type": "text", "text": text},
	}
	for _, url := range media {
		parts = append(parts, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{"url": url},
		})
	}
	return parts
}
