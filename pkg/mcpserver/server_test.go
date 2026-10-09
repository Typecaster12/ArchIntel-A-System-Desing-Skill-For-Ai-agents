package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sdskill/archintel/pkg/engine"
)

func TestMCPServerLifecycleAndTools(t *testing.T) {
	eng := engine.NewEngine(engine.EngineOptions{})
	server := NewServer(eng)
	ctx := context.Background()

	// 1. Prepare requests using typed structs
	initBytes, _ := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	listBytes, _ := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	callArgs, _ := json.Marshal(CallParams{
		Name: "recommend_architecture",
		Arguments: map[string]interface{}{
			"prompt": "An online store checkout with inventory reservation",
		},
	})
	callBytes, _ := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  callArgs,
	})

	var buf bytes.Buffer
	buf.Write(initBytes)
	buf.WriteString("\n")
	buf.Write(listBytes)
	buf.WriteString("\n")
	buf.Write(callBytes)
	buf.WriteString("\n")

	output := &bytes.Buffer{}

	if err := server.Serve(ctx, &buf, output); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSON-RPC responses, got %d", len(lines))
	}

	// Verify initialize response
	var initResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("failed to unmarshal init response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("unexpected init error: %v", initResp.Error)
	}

	// Verify tools/list response
	var listResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("failed to unmarshal list response: %v", err)
	}
	resMap, ok := listResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map in tools/list")
	}
	tools, ok := resMap["tools"].([]interface{})
	if !ok || len(tools) < 5 {
		t.Errorf("expected at least 5 tools listed, got %d", len(tools))
	}

	// Verify tools/call recommend_architecture response
	var callResp JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &callResp); err != nil {
		t.Fatalf("failed to unmarshal call response: %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("unexpected tool call error: %v", callResp.Error)
	}
}
