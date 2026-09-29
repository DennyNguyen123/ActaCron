package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"actacron/internal/domain"
)

type FunctionInvoker interface {
	ListFunctions() []*domain.FunctionMeta
	CallWithTrigger(ctx context.Context, target string, params interface{}, trigger string) (interface{}, error)
}

type Handler struct {
	invoker FunctionInvoker
}

func NewHandler(invoker FunctionInvoker) *Handler {
	return &Handler{invoker: invoker}
}

func (h *Handler) Handle(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		res := InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCaps{
				Tools: &ToolsCap{ListChanged: true},
			},
			ServerInfo: Implementation{
				Name:    "ActaCron",
				Version: "1.0.0",
			},
		}
		raw, _ := json.Marshal(res)
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  raw,
		}

	case "notifications/initialized":
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

	case "tools/list":
		funcs := h.invoker.ListFunctions()
		var tools []Tool

		for _, fn := range funcs {
			if !fn.IsMCP {
				continue
			}

			// Format safe tool name: pkg_name
			safeName := fmt.Sprintf("%s_%s", fn.Package, fn.Name)
			properties := make(map[string]PropertySchema)
			var required []string

			for _, p := range fn.Params {
				properties[p.Name] = PropertySchema{
					Type:        p.Type,
					Description: p.Description,
				}
				if p.Required {
					required = append(required, p.Name)
				}
			}

			tools = append(tools, Tool{
				Name:        safeName,
				Description: fn.Description,
				InputSchema: InputSchema{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
			})
		}

		raw, _ := json.Marshal(ToolsListResult{Tools: tools})
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  raw,
		}

	case "tools/call":
		var p CallToolParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32602, Message: "invalid params: " + err.Error()},
			}
		}

		// Map safe name back to "pkg/func"
		target := p.Name
		if idx := strings.Index(target, "_"); idx != -1 {
			target = target[:idx] + "/" + target[idx+1:]
		}

		res, err := h.invoker.CallWithTrigger(ctx, target, p.Arguments, "mcp")
		if err != nil {
			callRes := CallToolResult{
				IsError: true,
				Content: []ContentItem{
					{Type: "text", Text: "Error executing tool: " + err.Error()},
				},
			}
			raw, _ := json.Marshal(callRes)
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  raw,
			}
		}

		var text string
		if resBytes, err := json.Marshal(res); err == nil {
			text = string(resBytes)
		} else {
			text = fmt.Sprint(res)
		}

		callRes := CallToolResult{
			Content: []ContentItem{
				{Type: "text", Text: text},
			},
		}
		raw, _ := json.Marshal(callRes)
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  raw,
		}

	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func RunStdio(invoker FunctionInvoker) error {
	handler := NewHandler(invoker)
	scanner := bufio.NewScanner(os.Stdin)

	// Increase buffer size for large JSON payloads
	const maxCapacity = 10 * 1024 * 1024
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
			}
			bytes, _ := json.Marshal(resp)
			fmt.Println(string(bytes))
			continue
		}

		resp := handler.Handle(context.Background(), req)
		if resp.ID != nil {
			bytes, _ := json.Marshal(resp)
			fmt.Println(string(bytes))
		}
	}
	return scanner.Err()
}

func NewSSEHandler(invoker FunctionInvoker) http.Handler {
	handler := NewHandler(invoker)
	mux := http.NewServeMux()

	mux.HandleFunc("/mcp/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "event: endpoint\ndata: /mcp/messages\n\n")
		flusher.Flush()

		<-r.Context().Done()
	})

	mux.HandleFunc("/mcp/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Invalid JSON-RPC", http.StatusBadRequest)
			return
		}

		resp := handler.Handle(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	return mux
}
