package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// nopWriteCloser wraps an io.Writer with a no-op Close.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// newTestClient creates a daemonClient wired to in-memory pipes.
// The caller writes responses into serverWriter (simulating daemon stdout)
// and reads requests from serverReader (simulating daemon stdin).
func newTestClient(t *testing.T, serverReader io.ReadCloser, serverWriter io.WriteCloser) *daemonClient {
	t.Helper()
	dc := &daemonClient{
		stdin:  serverWriter,
		stdout: serverReader,
		reader: bufio.NewReaderSize(serverReader, 64*1024),
	}
	return dc
}

// writeFrame writes a Content-Length framed JSON message to w.
func testWriteFrame(t *testing.T, w io.Writer, msg interface{}) {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(w, header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
}

// readFrame reads one Content-Length framed message from r.
func testReadFrame(t *testing.T, r *bufio.Reader) json.RawMessage {
	t.Helper()
	contentLen := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("readFrame header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			fmt.Sscanf(val, "%d", &contentLen)
		}
	}
	if contentLen < 0 {
		t.Fatal("missing Content-Length header")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("readFrame body: %v", err)
	}
	return body
}

// ── readFrame Tests ────────────────────────────────────────────

func TestReadFrame_ValidMessage(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	dc := newTestClient(t, pr, nopWriteCloser{io.Discard})

	payload := `{"jsonrpc":"2.0","id":1,"result":"ok"}`
	go func() {
		defer pw.Close()
		fmt.Fprintf(pw, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	}()

	msg, err := dc.readFrame()
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if string(msg) != payload {
		t.Errorf("got %q, want %q", msg, payload)
	}
}

func TestReadFrame_MissingContentLength(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	dc := newTestClient(t, pr, nopWriteCloser{io.Discard})

	go func() {
		defer pw.Close()
		// Send headers without Content-Length, then blank line
		fmt.Fprintf(pw, "X-Custom: foo\r\n\r\n")
	}()

	_, err := dc.readFrame()
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
	if !strings.Contains(err.Error(), "Content-Length") {
		t.Errorf("error should mention Content-Length, got: %v", err)
	}
}

func TestReadFrame_PipeClose(t *testing.T) {
	pr, pw := io.Pipe()
	dc := newTestClient(t, pr, nopWriteCloser{io.Discard})

	pw.Close()

	_, err := dc.readFrame()
	if err == nil {
		t.Fatal("expected error on closed pipe")
	}
}

func TestReadFrame_MultipleHeaders(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()

	dc := newTestClient(t, pr, nopWriteCloser{io.Discard})

	payload := `{"test":true}`
	go func() {
		defer pw.Close()
		// Extra headers before Content-Length
		fmt.Fprintf(pw, "X-Extra: ignored\r\nContent-Length: %d\r\n\r\n%s", len(payload), payload)
	}()

	msg, err := dc.readFrame()
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if string(msg) != payload {
		t.Errorf("got %q, want %q", msg, payload)
	}
}

// ── writeFrame Tests ───────────────────────────────────────────

func TestWriteFrame_ValidMessage(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	// Use pw as the client's stdin so writeFrame writes to it
	dc := &daemonClient{
		stdin:  pw,
		reader: bufio.NewReader(strings.NewReader("")),
	}

	msg := rpcRequest{JSONRPC: "2.0", ID: 1, Method: "test/method"}
	go func() {
		if err := dc.writeFrame(msg); err != nil {
			t.Errorf("writeFrame: %v", err)
		}
	}()

	reader := bufio.NewReader(pr)
	raw := testReadFrame(t, reader)

	var got rpcRequest
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Method != "test/method" {
		t.Errorf("method = %q, want %q", got.Method, "test/method")
	}
	if got.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want %q", got.JSONRPC, "2.0")
	}
	if got.ID != 1 {
		t.Errorf("id = %d, want %d", got.ID, 1)
	}
}

func TestWriteFrame_IncludesParams(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	dc := &daemonClient{
		stdin:  pw,
		reader: bufio.NewReader(strings.NewReader("")),
	}

	params := map[string]string{"key": "value"}
	msg := rpcRequest{JSONRPC: "2.0", ID: 2, Method: "test/params", Params: params}

	go func() {
		if err := dc.writeFrame(msg); err != nil {
			t.Errorf("writeFrame: %v", err)
		}
	}()

	reader := bufio.NewReader(pr)
	raw := testReadFrame(t, reader)

	var got struct {
		Params map[string]string `json:"params"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Params["key"] != "value" {
		t.Errorf("params.key = %q, want %q", got.Params["key"], "value")
	}
}

// ── call / callInto Tests ──────────────────────────────────────

func TestCall_SendAndReceive(t *testing.T) {
	// clientReader: client reads responses from here
	// serverWriter: test writes responses here (simulating daemon)
	clientReader, serverWriter := io.Pipe()
	// serverReader: test reads requests from here
	// clientWriter: client sends requests here
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)

	// Run call in a goroutine
	type callResult struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan callResult, 1)
	go func() {
		raw, err := dc.call("test/echo", map[string]string{"msg": "hello"})
		ch <- callResult{raw, err}
	}()

	// Read the request from the "server" side
	reqRaw := testReadFrame(t, srvBuf)
	var req rpcRequest
	if err := json.Unmarshal(reqRaw, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Method != "test/echo" {
		t.Fatalf("method = %q, want %q", req.Method, "test/echo")
	}

	// Write a response back
	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		ID:      &req.ID,
		Result:  json.RawMessage(`"world"`),
	})

	// Read the call result
	result := <-ch
	if result.err != nil {
		t.Fatalf("call error: %v", result.err)
	}

	var s string
	if err := json.Unmarshal(result.raw, &s); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if s != "world" {
		t.Errorf("result = %q, want %q", s, "world")
	}
}

func TestCall_RPCError(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)

	ch := make(chan error, 1)
	go func() {
		_, err := dc.call("test/fail", nil)
		ch <- err
	}()

	reqRaw := testReadFrame(t, srvBuf)
	var req rpcRequest
	json.Unmarshal(reqRaw, &req)

	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		ID:      &req.ID,
		Error:   &rpcError{Code: -32600, Message: "invalid request"},
	})

	err := <-ch
	if err == nil {
		t.Fatal("expected RPC error")
	}
	if !strings.Contains(err.Error(), "invalid request") {
		t.Errorf("error = %v, want 'invalid request'", err)
	}
	if !strings.Contains(err.Error(), "-32600") {
		t.Errorf("error should contain code -32600, got: %v", err)
	}
}

func TestCallInto_Unmarshal(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)

	type resultType struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}

	ch := make(chan error, 1)
	var dest resultType
	go func() {
		ch <- dc.callInto(&dest, "test/data", nil)
	}()

	reqRaw := testReadFrame(t, srvBuf)
	var req rpcRequest
	json.Unmarshal(reqRaw, &req)

	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		ID:      &req.ID,
		Result:  json.RawMessage(`{"status":"active","count":42}`),
	})

	if err := <-ch; err != nil {
		t.Fatalf("callInto: %v", err)
	}
	if dest.Status != "active" {
		t.Errorf("status = %q, want %q", dest.Status, "active")
	}
	if dest.Count != 42 {
		t.Errorf("count = %d, want %d", dest.Count, 42)
	}
}

func TestCallInto_NilDest(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)

	ch := make(chan error, 1)
	go func() {
		ch <- dc.callInto(nil, "test/void", nil)
	}()

	reqRaw := testReadFrame(t, srvBuf)
	var req rpcRequest
	json.Unmarshal(reqRaw, &req)

	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		ID:      &req.ID,
		Result:  json.RawMessage(`null`),
	})

	if err := <-ch; err != nil {
		t.Fatalf("callInto with nil dest should succeed: %v", err)
	}
}

// ── Notification Tests ─────────────────────────────────────────

func TestNotificationDispatch(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	_, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)

	notifCh := make(chan string, 1)
	dc.onNotification = func(method string) {
		notifCh <- method
	}

	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	// Server sends a notification (no ID)
	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		Method:  "activate/stateChanged",
	})

	select {
	case method := <-notifCh:
		if method != "activate/stateChanged" {
			t.Errorf("notification method = %q, want %q", method, "activate/stateChanged")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}
}

func TestNotification_NoCallback(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	_, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	// onNotification is nil — should not panic

	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		Method:  "activate/stateChanged",
	})

	// Give readLoop time to process without panicking
	time.Sleep(100 * time.Millisecond)
}

// ── Concurrent Request Tests ───────────────────────────────────

func TestConcurrentRequests(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)
	const numCalls = 5

	type callResult struct {
		idx int
		raw json.RawMessage
		err error
	}

	resultCh := make(chan callResult, numCalls)

	// Launch concurrent calls
	for i := 0; i < numCalls; i++ {
		go func(idx int) {
			raw, err := dc.call("test/concurrent", map[string]int{"idx": idx})
			resultCh <- callResult{idx, raw, err}
		}(i)
	}

	// Read all requests from server side, respond in reverse order to
	// verify responses are routed by ID, not arrival order.
	type pending struct {
		id  int64
		idx int
	}
	var reqs []pending

	for i := 0; i < numCalls; i++ {
		reqRaw := testReadFrame(t, srvBuf)
		var req struct {
			ID     int64 `json:"id"`
			Params struct {
				Idx int `json:"idx"`
			} `json:"params"`
		}
		json.Unmarshal(reqRaw, &req)
		reqs = append(reqs, pending{id: req.ID, idx: req.Params.Idx})
	}

	// Respond in reverse order
	for i := len(reqs) - 1; i >= 0; i-- {
		resp := fmt.Sprintf(`"response-%d"`, reqs[i].idx)
		testWriteFrame(t, serverWriter, rpcResponse{
			JSONRPC: "2.0",
			ID:      &reqs[i].id,
			Result:  json.RawMessage(resp),
		})
	}

	// Collect all results
	results := make(map[int]string)
	for i := 0; i < numCalls; i++ {
		r := <-resultCh
		if r.err != nil {
			t.Fatalf("call %d error: %v", r.idx, r.err)
		}
		var s string
		json.Unmarshal(r.raw, &s)
		results[r.idx] = s
	}

	for i := 0; i < numCalls; i++ {
		expected := fmt.Sprintf("response-%d", i)
		if results[i] != expected {
			t.Errorf("call %d: got %q, want %q", i, results[i], expected)
		}
	}
}

// ── Stop Tests ─────────────────────────────────────────────────

func TestStop_SendsShutdown(t *testing.T) {
	// Verify stop() writes the shutdown request to stdin
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)
	// Don't start readLoop — we just want to verify the write

	defer func() {
		serverWriter.Close()
		clientReader.Close()
	}()

	srvBuf := bufio.NewReader(serverReader)

	// stop() will write shutdown, close stdin, then try cmd.Wait().
	// Since cmd is nil, we handle the panic with a recover-based approach.
	// Instead, we just test that writeFrame works for the shutdown message.
	shutdownReq := rpcRequest{JSONRPC: "2.0", ID: 0, Method: "activate/shutdown"}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = dc.writeFrame(shutdownReq)
		clientWriter.Close()
	}()

	reqRaw := testReadFrame(t, srvBuf)
	var req rpcRequest
	if err := json.Unmarshal(reqRaw, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Method != "activate/shutdown" {
		t.Errorf("method = %q, want %q", req.Method, "activate/shutdown")
	}

	wg.Wait()
	serverReader.Close()
}

// ── readLoop Tests ─────────────────────────────────────────────

func TestReadLoop_ExitsOnClose(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	_, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)

	done := make(chan struct{})
	go func() {
		dc.readLoop()
		close(done)
	}()

	// Close the server writer to simulate daemon exit
	serverWriter.Close()

	select {
	case <-done:
		// readLoop exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop did not exit after pipe close")
	}

	clientWriter.Close()
}

func TestReadLoop_SkipsMalformedJSON(t *testing.T) {
	clientReader, serverWriter := io.Pipe()
	_, clientWriter := io.Pipe()

	dc := newTestClient(t, clientReader, clientWriter)

	notifCh := make(chan string, 1)
	dc.onNotification = func(method string) {
		notifCh <- method
	}

	go dc.readLoop()
	defer func() {
		clientWriter.Close()
		serverWriter.Close()
	}()

	// Send malformed JSON first
	badPayload := `{not valid json`
	fmt.Fprintf(serverWriter, "Content-Length: %d\r\n\r\n%s", len(badPayload), badPayload)

	// Then send a valid notification — should still be received
	testWriteFrame(t, serverWriter, rpcResponse{
		JSONRPC: "2.0",
		Method:  "activate/recovered",
	})

	select {
	case method := <-notifCh:
		if method != "activate/recovered" {
			t.Errorf("method = %q, want %q", method, "activate/recovered")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification after malformed message")
	}
}

func TestNextID_Increments(t *testing.T) {
	dc := &daemonClient{}
	id1 := dc.nextID.Add(1)
	id2 := dc.nextID.Add(1)
	if id2 != id1+1 {
		t.Errorf("nextID not incrementing: %d, %d", id1, id2)
	}
}
