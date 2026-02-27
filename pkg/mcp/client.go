package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
)

// Client communicates with a single MCP server process over stdio.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	nextID  atomic.Int64
	mu      sync.Mutex
	pending map[int]chan Response

	done chan struct{} // closed when the reader goroutine exits
}

// NewClient creates a Client that will spawn the given command.
// The process is not started until Start is called.
func NewClient(command string, args []string) (*Client, error) {
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	return &Client{
		cmd:     cmd,
		stdin:   stdinPipe,
		stdout:  stdoutPipe,
		pending: make(map[int]chan Response),
		done:    make(chan struct{}),
	}, nil
}

// Start launches the MCP server process and performs the initialize handshake.
func (c *Client) Start(ctx context.Context) error {
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	// Start the background reader before sending any requests.
	go c.readLoop()

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    ClientCapabilities{},
		ClientInfo: ClientInfo{
			Name:    "glamdring",
			Version: "0.1.0",
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		_ = c.Close()
		return fmt.Errorf("initialize: %w", err)
	}

	// Send initialized notification (no ID, no response expected).
	notification := struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
	}{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notification)
	data = append(data, '\n')

	c.mu.Lock()
	_, err := c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		_ = c.Close()
		return fmt.Errorf("send initialized notification: %w", err)
	}

	return nil
}

// ListTools sends tools/list and returns the available tool definitions.
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	var result ToolsListResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool sends tools/call and returns the concatenated text content.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (string, bool, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	var result ToolCallResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return "", false, err
	}

	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			if text != "" {
				text += "\n"
			}
			text += block.Text
		}
	}
	return text, result.IsError, nil
}

// Close terminates the MCP server process.
func (c *Client) Close() error {
	// Close stdin to signal the server.
	_ = c.stdin.Close()

	if c.cmd.Process != nil {
		_ = c.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for the reader goroutine to finish.
	<-c.done

	// Wait for the process to exit.
	return c.cmd.Wait()
}

// Alive reports whether the server process is still running.
func (c *Client) Alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// call sends a JSON-RPC request and waits for the matching response.
func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := int(c.nextID.Add(1))

	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	// Register the pending response channel before writing.
	ch := make(chan Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	// Write the request.
	c.mu.Lock()
	_, err = c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("write request: %w", err)
	}

	// Wait for the response or context cancellation.
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && resp.Result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case <-c.done:
		return fmt.Errorf("mcp server process exited")
	}
}

// readLoop reads newline-delimited JSON-RPC responses from stdout and
// dispatches them to the pending request channels.
func (c *Client) readLoop() {
	defer close(c.done)

	scanner := bufio.NewScanner(c.stdout)
	// Allow up to 10MB per line for large tool results.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("mcp: failed to parse response: %v", err)
			continue
		}

		// Skip notifications (ID == 0 with no pending request).
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()

		if ok {
			ch <- resp
		}
	}

	// Server process exited or stdout closed. Unblock any pending callers.
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
}
