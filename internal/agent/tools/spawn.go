package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/local/picobot/internal/chat"
)

// SpawnRunner runs a subagent task in an isolated session and returns the result.
// requesterChannel and requesterChatID are used for any message tool sends from the subagent.
type SpawnRunner interface {
	RunSubagent(ctx context.Context, sessionKey string, task string, timeout time.Duration, requesterChannel, requesterChatID string) (string, error)
}

// SpawnTool spawns a background subagent that runs the task and announces the result to the requester.
type SpawnTool struct {
	hub    *chat.Hub
	runner SpawnRunner
	// Context set per-message by the agent loop
	channel string
	chatID  string
}

// NewSpawnTool creates a SpawnTool. runner must implement SpawnRunner (e.g. *agent.AgentLoop).
func NewSpawnTool(hub *chat.Hub, runner SpawnRunner) *SpawnTool {
	return &SpawnTool{hub: hub, runner: runner}
}

// SetContext sets the requester channel and chat id for announcement delivery.
func (t *SpawnTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *SpawnTool) Name() string        { return "spawn" }
func (t *SpawnTool) Description() string { return "Spawn a background subagent to run a task; the result will be announced to the chat when done." }

func (t *SpawnTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task description for the spawned agent",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Optional label for the subagent run",
			},
			"runTimeoutSeconds": map[string]interface{}{
				"type":        "number",
				"description": "Optional timeout in seconds (0 = default)",
			},
		},
		"required": []string{"task"},
	}
}

func (t *SpawnTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	task, _ := args["task"].(string)
	if task == "" {
		return "", fmt.Errorf("spawn: 'task' required")
	}
	if t.channel == "subagent" {
		return "", fmt.Errorf("spawn: not allowed from subagent sessions")
	}
	if t.runner == nil {
		return `{"status":"error","error":"spawn runner not configured"}`, nil
	}

	childSessionKey := "subagent:" + uuid.New().String()
	runID := uuid.New().String()
	timeout := 120 * time.Second
	if s, ok := args["runTimeoutSeconds"].(float64); ok && s > 0 {
		timeout = time.Duration(s) * time.Second
	}

	channel := t.channel
	chatID := t.chatID

	go func() {
		result, err := t.runner.RunSubagent(ctx, childSessionKey, task, timeout, channel, chatID)
		if err != nil {
			result = fmt.Sprintf("(error) %v", err)
		}
		announce := fmt.Sprintf("**Subagent result:**\n\n%s", result)
		select {
		case t.hub.Out <- chat.Outbound{Channel: channel, ChatID: chatID, Content: announce}:
		default:
			// outbound channel full; result is lost
		}
	}()

	out := map[string]interface{}{
		"status":          "accepted",
		"childSessionKey": childSessionKey,
		"runId":           runID,
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
