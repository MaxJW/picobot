package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/local/picobot/internal/chat"
	"github.com/local/picobot/internal/providers"
)

func TestProcessDirectWithStub(t *testing.T) {
	b := chat.NewHub(10)
	p := providers.NewStubProvider()

	ag := NewAgentLoop(b, p, p.GetDefaultModel(), 5, "", nil)

	resp, err := ag.ProcessDirect("hello", 1*time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == "" {
		t.Fatalf("expected response, got empty string")
	}
}

func TestRunSubagentWithStub(t *testing.T) {
	b := chat.NewHub(10)
	p := providers.NewStubProvider()
	ag := NewAgentLoop(b, p, p.GetDefaultModel(), 5, "", nil)

	ctx := context.Background()
	resp, err := ag.RunSubagent(ctx, "subagent:test-123", "what is 2+2?", 5*time.Second, "discord", "456")
	if err != nil {
		t.Fatalf("RunSubagent: %v", err)
	}
	if !strings.Contains(resp, "2+2") {
		t.Errorf("expected stub to echo task, got %q", resp)
	}
}
