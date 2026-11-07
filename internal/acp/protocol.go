package acp

import (
	"fmt"
	json "github.com/goccy/go-json"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("acp: rpc error %d: %s", e.Code, e.Message)
}

const (
	ErrCodeParse          = -32700
	ErrCodeInvalidReq     = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

const (
	ErrCodeCancelled = -32800
)

const (
	ErrCodeAuthRequired     = -32000
	ErrCodeResourceNotFound = -32002
)

func isNotification(data []byte) bool {
	var n struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(data, &n); err != nil {
		return true
	}
	return n.ID == nil
}

func isBatch(msg Request) bool {
	return false
}
