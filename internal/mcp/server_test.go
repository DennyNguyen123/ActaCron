package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"actacron/internal/domain"
	"actacron/internal/mcp"
)

type mockInvoker struct {
	funcs []*domain.FunctionMeta
}

func (m *mockInvoker) ListFunctions() []*domain.FunctionMeta {
	return m.funcs
}

func (m *mockInvoker) CallWithTrigger(ctx context.Context, target string, params interface{}, trigger string) (interface{}, error) {
	return map[string]interface{}{"result": "called " + target}, nil
}

func TestMCPToolsListAndCall(t *testing.T) {
	funcs := []*domain.FunctionMeta{
		{
			Name:        "get_weather",
			Package:     "utils",
			Description: "Get weather for city",
			IsMCP:       true,
			Params: []domain.ParamSchema{
				{Name: "city", Type: "string", Description: "Target city", Required: true},
			},
		},
		{
			Name:        "private_func",
			Package:     "utils",
			Description: "Not an MCP tool",
			IsMCP:       false,
		},
	}

	invoker := &mockInvoker{funcs: funcs}
	handler := mcp.NewHandler(invoker)

	// 1. Test initialize
	initReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
	}
	initResp := handler.Handle(context.Background(), initReq)
	if initResp.Error != nil {
		t.Fatalf("initialize failed: %v", initResp.Error)
	}

	// 2. Test tools/list
	listReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/list",
	}
	listResp := handler.Handle(context.Background(), listReq)
	if listResp.Error != nil {
		t.Fatalf("tools/list failed: %v", listResp.Error)
	}

	var listRes mcp.ToolsListResult
	if err := json.Unmarshal(listResp.Result, &listRes); err != nil {
		t.Fatalf("unmarshal tools/list failed: %v", err)
	}
	if len(listRes.Tools) != 1 || listRes.Tools[0].Name != "utils_get_weather" {
		t.Fatalf("expected 1 tool utils_get_weather, got %v", listRes.Tools)
	}

	// 3. Test tools/call
	callParams, _ := json.Marshal(map[string]interface{}{
		"name": "utils_get_weather",
		"arguments": map[string]interface{}{
			"city": "Tokyo",
		},
	})
	callReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  callParams,
	}
	callResp := handler.Handle(context.Background(), callReq)
	if callResp.Error != nil {
		t.Fatalf("tools/call failed: %v", callResp.Error)
	}

	var callRes mcp.CallToolResult
	if err := json.Unmarshal(callResp.Result, &callRes); err != nil {
		t.Fatalf("unmarshal callToolResult failed: %v", err)
	}
	if len(callRes.Content) == 0 || callRes.IsError {
		t.Fatalf("expected successful call content, got %v", callRes)
	}
}
