package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func frameMessage(t *testing.T, msg interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("frameMessage marshal: %v", err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data))
}

func TestTransportReadMessage(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "initialize",
		Params:  json.RawMessage(`{"foo":"bar"}`),
	}
	buf := bytes.NewBuffer(frameMessage(t, req))
	tr := NewTransport(buf, io.Discard)

	got, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.Method != "initialize" {
		t.Errorf("Method = %q, want %q", got.Method, "initialize")
	}
	if string(got.ID) != "1" {
		t.Errorf("ID = %s, want 1", got.ID)
	}
	if got.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, "2.0")
	}
	if string(got.Params) != `{"foo":"bar"}` {
		t.Errorf("Params = %s, want %s", got.Params, `{"foo":"bar"}`)
	}
}

func TestTransportReadMessageMultiple(t *testing.T) {
	req1 := Request{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "first"}
	req2 := Request{JSONRPC: "2.0", ID: json.RawMessage(`2`), Method: "second"}

	var buf bytes.Buffer
	buf.Write(frameMessage(t, req1))
	buf.Write(frameMessage(t, req2))
	tr := NewTransport(&buf, io.Discard)

	got1, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 1: %v", err)
	}
	if got1.Method != "first" {
		t.Errorf("msg1 Method = %q, want %q", got1.Method, "first")
	}

	got2, err := tr.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage 2: %v", err)
	}
	if got2.Method != "second" {
		t.Errorf("msg2 Method = %q, want %q", got2.Method, "second")
	}
}

func TestTransportReadMessageMissingContentLength(t *testing.T) {
	// Header section with no Content-Length, just the blank line terminator.
	input := "\r\n"
	tr := NewTransport(strings.NewReader(input), io.Discard)

	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
	if !strings.Contains(err.Error(), "missing Content-Length") {
		t.Errorf("error = %q, want it to mention missing Content-Length", err)
	}
}

func TestTransportReadMessageInvalidContentLength(t *testing.T) {
	input := "Content-Length: abc\r\n\r\n"
	tr := NewTransport(strings.NewReader(input), io.Discard)

	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatal("expected error for invalid Content-Length")
	}
	if !strings.Contains(err.Error(), "invalid Content-Length") {
		t.Errorf("error = %q, want it to mention invalid Content-Length", err)
	}
}

func TestTransportReadMessageInvalidJSON(t *testing.T) {
	body := "not json at all"
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	tr := NewTransport(strings.NewReader(input), io.Discard)

	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse request") {
		t.Errorf("error = %q, want it to mention parse request", err)
	}
}

func TestTransportReadMessageEOF(t *testing.T) {
	tr := NewTransport(strings.NewReader(""), io.Discard)

	_, err := tr.ReadMessage()
	if err == nil {
		t.Fatal("expected error on empty reader")
	}
}

func TestTransportWriteResponse(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(strings.NewReader(""), &buf)

	resp := SuccessResponse(json.RawMessage(`1`), map[string]string{"status": "ok"})
	if err := tr.WriteResponse(resp); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "Content-Length: ") {
		t.Fatalf("output missing Content-Length header: %q", output)
	}

	// Parse back via a fresh transport
	readTr := NewTransport(bytes.NewBufferString(output), io.Discard)
	// ReadMessage parses into Request, but the JSON structure is still valid.
	// Instead, parse manually.
	_ = readTr

	// Manually split header and body
	parts := strings.SplitN(output, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected header + body, got %d parts", len(parts))
	}

	var got Response
	if err := json.Unmarshal([]byte(parts[1]), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", got.JSONRPC, "2.0")
	}
	if string(got.ID) != "1" {
		t.Errorf("ID = %s, want 1", got.ID)
	}
	if got.Error != nil {
		t.Errorf("unexpected error in response: %v", got.Error)
	}
}

func TestTransportWriteNotification(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(strings.NewReader(""), &buf)

	notif := StateChangedNotification()
	if err := tr.WriteNotification(notif); err != nil {
		t.Fatalf("WriteNotification: %v", err)
	}

	parts := strings.SplitN(buf.String(), "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected header + body, got %d parts", len(parts))
	}

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(parts[1]), &got); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if got["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", got["jsonrpc"])
	}
	if got["method"] != "activate/stateChanged" {
		t.Errorf("method = %v, want activate/stateChanged", got["method"])
	}
	if _, hasID := got["id"]; hasID {
		t.Error("notification should not have id field")
	}
}

func TestTransportWriteResponseError(t *testing.T) {
	var buf bytes.Buffer
	tr := NewTransport(strings.NewReader(""), &buf)

	resp := ErrorResponse(json.RawMessage(`2`), ErrCodeMethodNotFound, "method not found")
	if err := tr.WriteResponse(resp); err != nil {
		t.Fatalf("WriteResponse: %v", err)
	}

	parts := strings.SplitN(buf.String(), "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected header + body, got %d parts", len(parts))
	}

	var got Response
	if err := json.Unmarshal([]byte(parts[1]), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Error == nil {
		t.Fatal("expected error in response")
	}
	if got.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("error code = %d, want %d", got.Error.Code, ErrCodeMethodNotFound)
	}
	if got.Error.Message != "method not found" {
		t.Errorf("error message = %q, want %q", got.Error.Message, "method not found")
	}
}

func TestSuccessResponse(t *testing.T) {
	resp := SuccessResponse(json.RawMessage(`42`), "hello")
	if string(resp.ID) != "42" {
		t.Errorf("ID = %s, want 42", resp.ID)
	}
	if resp.Result != "hello" {
		t.Errorf("Result = %v, want hello", resp.Result)
	}
	if resp.Error != nil {
		t.Error("Error should be nil for success response")
	}
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse(json.RawMessage(`3`), ErrCodeParse, "parse error")
	if string(resp.ID) != "3" {
		t.Errorf("ID = %s, want 3", resp.ID)
	}
	if resp.Result != nil {
		t.Errorf("Result should be nil, got %v", resp.Result)
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != ErrCodeParse {
		t.Errorf("Code = %d, want %d", resp.Error.Code, ErrCodeParse)
	}
	if resp.Error.Message != "parse error" {
		t.Errorf("Message = %q, want %q", resp.Error.Message, "parse error")
	}
}

func TestStateChangedNotification(t *testing.T) {
	notif := StateChangedNotification()
	if notif.Method != "activate/stateChanged" {
		t.Errorf("Method = %q, want %q", notif.Method, "activate/stateChanged")
	}
	if notif.Params != nil {
		t.Errorf("Params = %v, want nil", notif.Params)
	}
}

func TestRPCErrorImplementsError(t *testing.T) {
	rpcErr := &RPCError{Code: ErrCodeParse, Message: "parse error"}
	var err error = rpcErr // verify it satisfies the error interface

	got := err.Error()
	want := "rpc error -32700: parse error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestTransportRoundTrip(t *testing.T) {
	// Client writes a request, server reads it, responds, client reads response.
	serverFromClient, clientToServer := io.Pipe()
	clientFromServer, serverToClient := io.Pipe()

	clientTr := NewTransport(clientFromServer, clientToServer)
	serverTr := NewTransport(serverFromClient, serverToClient)

	errCh := make(chan error, 2)

	// Server goroutine: read request, send response
	go func() {
		req, err := serverTr.ReadMessage()
		if err != nil {
			errCh <- fmt.Errorf("server read: %w", err)
			return
		}
		resp := SuccessResponse(req.ID, map[string]string{"result": "done"})
		if err := serverTr.WriteResponse(resp); err != nil {
			errCh <- fmt.Errorf("server write: %w", err)
			return
		}
		errCh <- nil
	}()

	// Client goroutine: send request, read response
	go func() {
		req := Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`99`),
			Method:  "test/roundTrip",
			Params:  json.RawMessage(`{"key":"value"}`),
		}
		data, _ := json.Marshal(req)
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
		if _, err := io.WriteString(clientToServer, header); err != nil {
			errCh <- fmt.Errorf("client write header: %w", err)
			return
		}
		if _, err := clientToServer.Write(data); err != nil {
			errCh <- fmt.Errorf("client write body: %w", err)
			return
		}

		// Read the response
		respReq, err := clientTr.ReadMessage()
		if err != nil {
			errCh <- fmt.Errorf("client read: %w", err)
			return
		}

		// The response is parsed as a Request struct (since ReadMessage returns *Request),
		// but we can verify the raw JSON contains our result.
		_ = respReq
		errCh <- nil
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}
