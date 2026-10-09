package mcpserver

// ToolDefinition describes a tool exposed over the Model Context Protocol.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// GetToolDefinitions returns the schema catalog for all architectural intelligence tools.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name: "analyze_requirements",
			Description: "Analyzes raw requirements, normalizes traffic/scale figures, and identifies critical unstated assumptions.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Natural language description of the system or application to build.",
					},
					"domain_hint": map[string]interface{}{
						"type":        "string",
						"description": "Optional business domain hint (e.g., ECOMMERCE, FINTECH, IOT, ANALYTICS).",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name: "classify_workload",
			Description: "Evaluates multi-label workload traits (TransactionHeavy, ReadHeavy, WriteHeavy, LatencySensitive, etc.) with calibrated confidence scores.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "System requirement prompt to classify.",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name: "recommend_architecture",
			Description: "Executes full architectural decision pipeline and returns the schema-validated ArchitectureContract with component boundaries, transaction rules, queue configurations, and concrete constraints.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Complete requirement description for the system.",
					},
					"preferred_stack": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Preferred programming languages, frameworks, or databases.",
					},
					"stated_traffic": map[string]interface{}{
						"type":        "string",
						"description": "Estimated user or throughput scale (e.g. '500 RPS', '100k DAU').",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name: "evaluate_tradeoffs",
			Description: "Compares candidate architectural patterns and generates explicit justifications for why specific alternatives were rejected.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Requirements prompt to evaluate trade-offs against.",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			Name: "validate_architecture",
			Description: "Validates agent-proposed code files, directory structures, or design choices against the active ArchitectureContract.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"contract": map[string]interface{}{
						"type":        "object",
						"description": "The active ArchitectureContract to validate against.",
					},
					"proposal": map[string]interface{}{
						"type":        "object",
						"description": "Proposed files, code snippets, and architectural flags.",
					},
				},
				"required": []string{"contract", "proposal"},
			},
		},
	}
}
