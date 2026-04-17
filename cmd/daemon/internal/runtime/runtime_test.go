package runtime_test

import (
	"testing"

	"github.com/loom/daemon/internal/runtime"
)

func TestClaudeRuntime(t *testing.T) {
	rt, err := runtime.NewClaudeRuntime(runtime.Config{
		CLIPath:    "echo",
		WorkingDir: ".",
	})
	if err != nil {
		t.Skipf("Claude not available: %v", err)
	}

	if rt.Name() != "claude-code" {
		t.Errorf("expected name 'claude-code', got %s", rt.Name())
	}

	tools := rt.ListTools()
	if len(tools) == 0 {
		t.Error("expected tools, got none")
	}
}

func TestRuntimeFactory(t *testing.T) {
	tests := []struct {
		name        string
		runtimeType string
		wantErr     bool
	}{
		{
			name:        "claude runtime",
			runtimeType: "claude",
			wantErr:     false,
		},
		{
			name:        "unknown runtime",
			runtimeType: "unknown",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := runtime.Factory(tt.runtimeType, runtime.Config{})
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Skipf("runtime not available: %v", err)
			}
			if rt == nil {
				t.Error("expected runtime, got nil")
			}
		})
	}
}
