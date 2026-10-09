package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sdskill/archintel/pkg/engine"
	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/validator"
)

// JSONRPCRequest represents an incoming JSON-RPC 2.0 message.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 message.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError is a standard JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CallParams represents the parameters for tools/call.
type CallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ContentItem represents a text block returned by an MCP tool.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallResult represents the tool execution response in MCP.
type CallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Server handles MCP communication over standard streams.
type Server struct {
	engine *engine.Engine
}

// NewServer creates a new MCP Server backed by the core engine.
func NewServer(eng *engine.Engine) *Server {
	return &Server{
		engine: eng,
	}
}

// Serve reads JSON-RPC requests line by line from reader and writes responses to writer.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(w, &JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			s.writeResponse(w, resp)
		}
	}

	return scanner.Err()
}

// handleRequest routes incoming JSON-RPC methods.
func (s *Server) handleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "archintel-mcp",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			},
		}

	case "notifications/initialized":
		// MCP client notification, no response required
		return nil

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": GetToolDefinitions(),
			},
		}

	case "tools/call":
		var params CallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: -32602, Message: "Invalid params"},
			}
		}

		result := s.executeTool(ctx, params.Name, params.Arguments)
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		}
	}
}

// executeTool executes the requested tool via the engine and returns a CallResult.
func (s *Server) executeTool(ctx context.Context, name string, args map[string]interface{}) CallResult {
	switch name {
	case "analyze_requirements":
		prompt, _ := args["prompt"].(string)
		domainHint, _ := args["domain_hint"].(string)

		res, err := s.engine.AnalyzeRequirements(model.RawRequirement{
			Prompt:     prompt,
			DomainHint: domainHint,
		})
		if err != nil {
			return errorCallResult(err)
		}
		return jsonCallResult(res)

	case "classify_workload":
		prompt, _ := args["prompt"].(string)
		res, err := s.engine.ClassifyWorkload(ctx, model.RawRequirement{
			Prompt: prompt,
		})
		if err != nil {
			return errorCallResult(err)
		}
		return jsonCallResult(res)

	case "recommend_architecture":
		prompt, _ := args["prompt"].(string)
		statedTraffic, _ := args["stated_traffic"].(string)

		var stack []string
		if sList, ok := args["preferred_stack"].([]interface{}); ok {
			for _, item := range sList {
				if str, ok := item.(string); ok {
					stack = append(stack, str)
				}
			}
		}

		res, err := s.engine.RecommendArchitecture(ctx, model.RawRequirement{
			Prompt:         prompt,
			PreferredStack: stack,
			StatedTraffic:  statedTraffic,
		})
		if err != nil {
			return errorCallResult(err)
		}
		return jsonCallResult(res)

	case "evaluate_tradeoffs":
		prompt, _ := args["prompt"].(string)
		res, err := s.engine.EvaluateTradeoffs(ctx, model.RawRequirement{
			Prompt: prompt,
		})
		if err != nil {
			return errorCallResult(err)
		}
		return jsonCallResult(res)

	case "validate_architecture":
		contractBytes, err := json.Marshal(args["contract"])
		if err != nil {
			return errorCallResult(fmt.Errorf("invalid contract: %w", err))
		}
		var archContract model.ArchitectureContract
		if err := json.Unmarshal(contractBytes, &archContract); err != nil {
			return errorCallResult(fmt.Errorf("malformed contract: %w", err))
		}

		proposalBytes, err := json.Marshal(args["proposal"])
		if err != nil {
			return errorCallResult(fmt.Errorf("invalid proposal: %w", err))
		}
		var proposal validator.ImplementationProposal
		if err := json.Unmarshal(proposalBytes, &proposal); err != nil {
			return errorCallResult(fmt.Errorf("malformed proposal: %w", err))
		}

		report := s.engine.ValidateArchitecture(&archContract, proposal)
		return jsonCallResult(report)

	default:
		return errorCallResult(fmt.Errorf("unknown tool: %s", name))
	}
}

func jsonCallResult(v interface{}) CallResult {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorCallResult(err)
	}
	return CallResult{
		Content: []ContentItem{
			{Type: "text", Text: string(bytes)},
		},
	}
}

func errorCallResult(err error) CallResult {
	return CallResult{
		IsError: true,
		Content: []ContentItem{
			{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
		},
	}
}

func (s *Server) writeResponse(w io.Writer, resp *JSONRPCResponse) {
	bytes, err := json.Marshal(resp)
	if err == nil {
		w.Write(bytes)
		w.Write([]byte("\n"))
	}
}
