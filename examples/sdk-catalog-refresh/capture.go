// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
package sdkcatalogrefresh

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"unicode/utf8"
)

const MaxFrameBytes = 64 << 10
const MaxCaptureBytes = 1 << 20
const MaxPages = 16

type Exchange struct {
	RequestID       string `json:"request_id"`
	Request         []byte `json:"original_request_bytes"`
	Response        []byte `json:"original_response_bytes,omitempty"`
	ResponsePresent bool   `json:"response_observed"`
	RequestWritten  bool   `json:"request_write_completed"`
}

type CaptureFailure struct {
	Code     string `json:"code"`
	Frame    []byte `json:"original_observed_bytes,omitempty"`
	Complete bool   `json:"complete_frame_observed"`
}

type capture struct {
	exchanges []Exchange
	pending   map[string]int
	writes    map[string]chan struct{}
	bytes     int
	failure   *CaptureFailure
}

type capturedIO struct {
	conn       io.ReadWriteCloser
	reader     *bufio.Reader
	readMu     sync.Mutex
	readBuffer []byte
	writeMu    sync.Mutex
	mu         sync.Mutex
	active     *capture
	closed     bool
	done       chan struct{}
	closeOnce  sync.Once
	closeErr   error
	onClose    func() error
	onChanged  func() error
}

func newCapturedIO(conn io.ReadWriteCloser, onClose, onChanged func() error) *capturedIO {
	return &capturedIO{conn: conn, reader: bufio.NewReaderSize(conn, MaxFrameBytes+1), done: make(chan struct{}), onClose: onClose, onChanged: onChanged}
}

// SDK writes may block before its context handling regains control. Closing the
// owned IO interrupts those writes. The returned stop function joins this
// watcher, so a completed operation leaves no cancellation goroutine behind.
func (w *capturedIO) closeOnCancellation(ctx context.Context) func() {
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			_ = w.Close()
		case <-w.done:
		case <-stop:
		}
	}()
	return func() { close(stop); <-joined }
}

func (w *capturedIO) begin() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("owned connection is closed")
	}
	if w.active != nil {
		return errors.New("capture already active")
	}
	w.active = &capture{exchanges: []Exchange{}, pending: map[string]int{}, writes: map[string]chan struct{}{}}
	return nil
}

func cloneExchanges(values []Exchange) []Exchange {
	out := make([]Exchange, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Request, out[i].Response = bytes.Clone(value.Request), bytes.Clone(value.Response)
	}
	return out
}

func (w *capturedIO) finish() ([]Exchange, *CaptureFailure, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active == nil {
		return []Exchange{}, nil, w.closed
	}
	c := w.active
	w.active = nil
	var failure *CaptureFailure
	if c.failure != nil {
		failure = &CaptureFailure{c.failure.Code, bytes.Clone(c.failure.Frame), c.failure.Complete}
	}
	return cloneExchanges(c.exchanges), failure, w.closed
}

func (w *capturedIO) latest(index int) (Exchange, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active == nil || len(w.active.exchanges) != index+1 || !w.active.exchanges[index].ResponsePresent {
		return Exchange{}, errors.New("SDK result lacks its fresh original request/response pair")
	}
	return cloneExchanges(w.active.exchanges[index : index+1])[0], nil
}

func (w *capturedIO) fail(code string, frame []byte, complete bool) {
	if len(frame) > MaxFrameBytes {
		frame, complete = frame[:MaxFrameBytes], false
	}
	w.mu.Lock()
	if w.active != nil && w.active.failure == nil {
		w.active.failure = &CaptureFailure{code, bytes.Clone(frame), complete}
	}
	w.mu.Unlock()
	_ = w.Close()
}

func (w *capturedIO) Close() error {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		if err := w.onClose(); err != nil {
			w.closeErr = err
		}
		close(w.done)
		if err := w.conn.Close(); w.closeErr == nil {
			w.closeErr = err
		}
	})
	return w.closeErr
}

func envelope(raw []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return nil, errors.New("frame is not valid UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	start, err := d.Token()
	if err != nil || start != json.Delim('{') {
		return nil, errors.New("one JSON-RPC object required")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, errors.New("invalid envelope key")
		}
		if _, exists := fields[key]; exists {
			return nil, errors.New("duplicate envelope key")
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		fields[key] = value
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') {
		return nil, errors.New("unterminated envelope")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("trailing envelope data")
	}
	return fields, nil
}

func integerID(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	value, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || strconv.FormatInt(value, 10) != string(raw) {
		return "", false
	}
	return string(raw), true
}

func (w *capturedIO) observe(raw []byte, outgoing bool) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	fields, err := envelope(raw)
	if err != nil {
		return err
	}
	var method string
	if value := fields["method"]; value != nil && json.Unmarshal(value, &method) != nil {
		return errors.New("method is not a string")
	}
	if !outgoing && method == "notifications/tools/list_changed" {
		// The epoch changes before these bytes reach SDK asynchronous dispatch.
		if err := w.onChanged(); err != nil {
			return err
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("owned connection closed during frame processing")
	}
	c := w.active
	if c == nil {
		return nil
	}
	if c.bytes+len(raw) > MaxCaptureBytes {
		return errors.New("capture byte bound exceeded")
	}
	c.bytes += len(raw)
	if outgoing && method == "tools/list" {
		id, ok := integerID(fields["id"])
		if !ok {
			return errors.New("expected SDK-generated integer request ID")
		}
		if len(c.exchanges) >= MaxPages {
			return errors.New("page request bound exceeded")
		}
		if _, duplicate := c.pending[id]; duplicate {
			return errors.New("request ID reused in capture")
		}
		c.pending[id] = len(c.exchanges)
		c.writes[id] = make(chan struct{})
		c.exchanges = append(c.exchanges, Exchange{RequestID: id, Request: bytes.Clone(raw)})
	} else if !outgoing && method == "" {
		id, ok := integerID(fields["id"])
		if !ok {
			return nil
		}
		if index, found := c.pending[id]; found {
			if c.exchanges[index].ResponsePresent {
				return errors.New("duplicate response for captured request")
			}
			c.exchanges[index].Response = bytes.Clone(raw)
			c.exchanges[index].ResponsePresent = true
		}
	}
	return nil
}

func (w *capturedIO) responseWriteBarrier(raw []byte) <-chan struct{} {
	fields, err := envelope(raw)
	if err != nil || fields["method"] != nil {
		return nil
	}
	id, ok := integerID(fields["id"])
	if !ok {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active == nil {
		return nil
	}
	return w.active.writes[id]
}

func (w *capturedIO) writeCompleted(raw []byte) {
	fields, err := envelope(raw)
	if err != nil {
		return
	}
	id, ok := integerID(fields["id"])
	if !ok {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active == nil {
		return
	}
	if index, found := w.active.pending[id]; found && !w.active.exchanges[index].RequestWritten {
		w.active.exchanges[index].RequestWritten = true
		close(w.active.writes[id])
	}
}

func (w *capturedIO) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.readMu.Lock()
	defer w.readMu.Unlock()
	if len(w.readBuffer) == 0 {
		frame, err := w.reader.ReadSlice('\n')
		if err != nil {
			code := "IO_READ_FAILED"
			if errors.Is(err, bufio.ErrBufferFull) {
				code = "FRAME_SIZE_EXCEEDED"
			} else if len(frame) > 0 {
				code = "UNTERMINATED_FRAME"
			}
			w.fail(code, frame, false)
			return 0, err
		}
		raw := frame[:len(frame)-1]
		if len(raw) > MaxFrameBytes {
			w.fail("FRAME_SIZE_EXCEEDED", raw, true)
			return 0, errors.New("frame byte bound exceeded")
		}
		if err := w.observe(raw, false); err != nil {
			w.fail("INCOMING_FRAME_REFUSED", raw, true)
			return 0, err
		}
		if barrier := w.responseWriteBarrier(raw); barrier != nil {
			select {
			case <-barrier:
			case <-w.done:
				return 0, errors.New("connection closed before request write completed")
			}
		}
		w.readBuffer = bytes.Clone(frame)
	}
	n := copy(p, w.readBuffer)
	w.readBuffer = w.readBuffer[n:]
	return n, nil
}

func (w *capturedIO) Write(p []byte) (int, error) {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if len(p) == 0 || p[len(p)-1] != '\n' || bytes.Count(p, []byte{'\n'}) != 1 || len(p)-1 > MaxFrameBytes {
		w.fail("OUTGOING_FRAME_REFUSED", p, false)
		return 0, errors.New("one bounded newline-delimited SDK message required")
	}
	if err := w.observe(p[:len(p)-1], true); err != nil {
		w.fail("OUTGOING_FRAME_REFUSED", p[:len(p)-1], true)
		return 0, err
	}
	written := 0
	for written < len(p) {
		n, err := w.conn.Write(p[written:])
		if n < 0 || n > len(p)-written || (n == 0 && err == nil) {
			err = io.ErrShortWrite
		}
		if n > 0 && n <= len(p)-written {
			written += n
		}
		if err != nil {
			w.fail("IO_WRITE_FAILED", nil, false)
			return written, fmt.Errorf("write original SDK frame: %w", err)
		}
	}
	w.writeCompleted(p[:len(p)-1])
	return written, nil
}
