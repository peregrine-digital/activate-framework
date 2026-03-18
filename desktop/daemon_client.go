package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// daemonClient is a JSON-RPC 2.0 client that communicates with the
// `activate serve --stdio` daemon over Content-Length framed stdio.
type daemonClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	reader  *bufio.Reader
	nextID  atomic.Int64
	mu      sync.Mutex // serializes writes
	pending sync.Map   // id → chan *rpcResponse

	// onNotification is called when the daemon pushes a notification
	onNotification func(method string)
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// startDaemon spawns `activate serve --stdio` and returns a connected client.
func startDaemon(binPath, projectDir string, env []string) (*daemonClient, error) {
	cmd := exec.Command(binPath, "serve", "--stdio")
	cmd.Dir = projectDir
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	dc := &daemonClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReaderSize(stdout, 64*1024),
	}

	go dc.readLoop()

	return dc, nil
}

// call sends a JSON-RPC request and waits for the response (30s timeout).
func (dc *daemonClient) call(method string, params interface{}) (json.RawMessage, error) {
	id := dc.nextID.Add(1)

	ch := make(chan *rpcResponse, 1)
	dc.pending.Store(id, ch)
	defer dc.pending.Delete(id)

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := dc.writeFrame(req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request %s timed out", method)
	}
}

// callInto sends a request and unmarshals the result into dest.
func (dc *daemonClient) callInto(dest interface{}, method string, params interface{}) error {
	raw, err := dc.call(method, params)
	if err != nil {
		return err
	}
	if dest != nil && len(raw) > 0 {
		return json.Unmarshal(raw, dest)
	}
	return nil
}

// writeFrame sends a Content-Length framed JSON message.
func (dc *daemonClient) writeFrame(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(dc.stdin, header); err != nil {
		return err
	}
	_, err = dc.stdin.Write(body)
	return err
}

// readLoop reads Content-Length framed messages from the daemon.
func (dc *daemonClient) readLoop() {
	for {
		msg, err := dc.readFrame()
		if err != nil {
			return // daemon exited or pipe closed
		}

		var resp rpcResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}

		if resp.ID != nil {
			// Response to a request
			if ch, ok := dc.pending.Load(*resp.ID); ok {
				ch.(chan *rpcResponse) <- &resp
			}
		} else if resp.Method != "" {
			// Notification from daemon
			if dc.onNotification != nil {
				dc.onNotification(resp.Method)
			}
		}
	}
}

// readFrame reads a single Content-Length framed message.
func (dc *daemonClient) readFrame() ([]byte, error) {
	contentLength := -1

	// Read headers
	for {
		line, err := dc.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(val)
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(dc.reader, body); err != nil {
		return nil, err
	}

	return body, nil
}

// stop gracefully shuts down the daemon.
func (dc *daemonClient) stop() {
	// Send shutdown request (best-effort)
	_ = dc.writeFrame(rpcRequest{JSONRPC: "2.0", ID: 0, Method: "activate/shutdown"})

	dc.stdin.Close()

	// Wait briefly for clean exit, then force kill
	done := make(chan error, 1)
	go func() { done <- dc.cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		dc.cmd.Process.Kill()
		<-done
	}
}
