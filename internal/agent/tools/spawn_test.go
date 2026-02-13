package tools

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/local/picobot/internal/chat"
)

type mockSpawnRunner struct {
	mu        sync.Mutex
	lastTask  string
	lastKey   string
	runResult string
	runErr    error
}

func (m *mockSpawnRunner) RunSubagent(ctx context.Context, sessionKey string, task string, timeout time.Duration, requesterChannel, requesterChatID string) (string, error) {
	m.mu.Lock()
	m.lastTask = task
	m.lastKey = sessionKey
	m.mu.Unlock()
	return m.runResult, m.runErr
}

func TestSpawnTool_Execute_ReturnsAccepted(t *testing.T) {
	hub := chat.NewHub(10)
	runner := &mockSpawnRunner{runResult: "subagent done"}
	tool := NewSpawnTool(hub, runner)
	tool.SetContext("discord", "123")

	result, err := tool.Execute(context.Background(), map[string]interface{}{"task": "do something"})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if out["status"] != "accepted" {
		t.Errorf("expected status accepted, got %v", out["status"])
	}
	if out["childSessionKey"] == nil || out["runId"] == nil {
		t.Errorf("expected childSessionKey and runId in response")
	}
}

func TestSpawnTool_Execute_RejectsFromSubagent(t *testing.T) {
	hub := chat.NewHub(10)
	runner := &mockSpawnRunner{}
	tool := NewSpawnTool(hub, runner)
	tool.SetContext("subagent", "abc-123")

	_, err := tool.Execute(context.Background(), map[string]interface{}{"task": "do something"})
	if err == nil {
		t.Fatal("expected error when spawning from subagent")
	}
	if err.Error() != "spawn: not allowed from subagent sessions" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpawnTool_Execute_RequiresTask(t *testing.T) {
	hub := chat.NewHub(10)
	runner := &mockSpawnRunner{}
	tool := NewSpawnTool(hub, runner)
	tool.SetContext("discord", "123")

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error when task is missing")
	}
}

func TestSpawnTool_Execute_CallsRunner(t *testing.T) {
	hub := chat.NewHub(10)
	runner := &mockSpawnRunner{runResult: "done"}
	tool := NewSpawnTool(hub, runner)
	tool.SetContext("discord", "123")

	_, err := tool.Execute(context.Background(), map[string]interface{}{"task": "my task"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.lastTask != "my task" {
		t.Errorf("runner lastTask = %q, want %q", runner.lastTask, "my task")
	}
	if runner.lastKey == "" || runner.lastKey[:9] != "subagent:" {
		t.Errorf("runner lastKey = %q, want subagent:...", runner.lastKey)
	}
}
