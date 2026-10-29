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
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type peerRequest struct {
	ID     json.RawMessage         `json:"id"`
	Method string                  `json:"method"`
	Params struct{ Cursor string } `json:"params"`
}

type peer struct {
	conn      net.Conn
	ctx       context.Context
	mu        sync.Mutex
	requests  [][]byte
	responses [][]byte
	written   chan struct{}
	done      chan error
}

func (p *peer) respond(req peerRequest, result string) error {
	raw := []byte(`{ "jsonrpc" : "2.0", "id" : ` + string(req.ID) + `, "result" : ` + result + ` }`)
	if _, err := p.conn.Write(append(bytes.Clone(raw), '\n')); err != nil {
		return err
	}
	p.mu.Lock()
	p.responses = append(p.responses, raw)
	p.mu.Unlock()
	p.written <- struct{}{}
	return nil
}

func (p *peer) rpcError(req peerRequest, code int64) error {
	raw := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"error":{"code":%d,"message":"synthetic tools/list failure"}}`, req.ID, code))
	if _, err := p.conn.Write(append(bytes.Clone(raw), '\n')); err != nil {
		return err
	}
	p.mu.Lock()
	p.responses = append(p.responses, raw)
	p.mu.Unlock()
	p.written <- struct{}{}
	return nil
}

func (p *peer) notify() error {
	_, err := io.WriteString(p.conn, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\",\"params\":{}}\n")
	return err
}

func newPeer(t *testing.T, handle func(*peer, int, peerRequest) error) (*Client, *peer) {
	t.Helper()
	return newPeerWithConn(t, handle, func(c net.Conn) io.ReadWriteCloser { return c })
}

func newPeerWithConn(t *testing.T, handle func(*peer, int, peerRequest) error, wrap func(net.Conn) io.ReadWriteCloser) (*Client, *peer) {
	t.Helper()
	left, right := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	p := &peer{conn: right, ctx: ctx, written: make(chan struct{}, MaxPages+4), done: make(chan error, 1)}
	go func() {
		defer right.Close()
		reader := bufio.NewReader(right)
		number := 0
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				p.done <- err
				return
			}
			var req peerRequest
			if err := json.Unmarshal(line, &req); err != nil {
				p.done <- err
				return
			}
			if len(req.ID) == 0 {
				continue
			}
			if req.Method == "initialize" {
				result := `{"protocolVersion":"2025-11-25","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"controlled-test-peer","version":"1"}}`
				if _, err := io.WriteString(right, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":`+result+"}\n"); err != nil {
					p.done <- err
					return
				}
				continue
			}
			if req.Method != "tools/list" {
				p.done <- fmt.Errorf("unexpected SDK request %s", req.Method)
				return
			}
			p.mu.Lock()
			p.requests = append(p.requests, bytes.TrimSuffix(bytes.Clone(line), []byte{'\n'}))
			p.mu.Unlock()
			err = handle(p, number, req)
			number++
			if err != nil {
				p.done <- err
				return
			}
		}
	}()
	c, err := Connect(ctx, wrap(left), []byte("prior opaque catalog"))
	if err != nil {
		cancel()
		left.Close()
		right.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = c.Close()
		_ = right.Close()
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
			t.Error("test peer did not exit")
		}
	})
	return c, p
}

func requirePreserved(t *testing.T, c *Client, report Report) {
	t.Helper()
	if report.Receipt == nil || report.Receipt.State != "RETAINED" || !bytes.Equal(c.Snapshot().Bytes, []byte("prior opaque catalog")) || report.Receipt.BeforeVersion != report.Receipt.AfterVersion || report.Receipt.BeforeSHA256 != report.Receipt.AfterSHA256 {
		t.Fatalf("stored state was not retained: %+v", report)
	}
}

func tool(name string) string { return `{"name":"` + name + `","inputSchema":{"type":"object"}}` }

func TestActualSDKWholeCatalogAndRowVisibility(t *testing.T) {
	for _, tc := range []struct {
		name       string
		results    []string
		state      string
		visibility string
	}{
		{"valid", []string{`{"tools":[` + tool(`\u0061lpha`) + `]}`}, "APPLIED", "SAME_ORDERED_NAMES"},
		{"empty", []string{`{"tools":[]}`}, "APPLIED", "SAME_ORDERED_NAMES"},
		{"null-row", []string{`{"tools":[null]}`}, "RETAINED", ""},
		{"SDK-filtered-header", []string{`{"tools":[{"name":"filtered","inputSchema":{"type":"object","properties":{"items":{"type":"array","x-mcp-header":"Items"}}}}]}`}, "RETAINED", "SDK_ROW_MISSING"},
		{"two-pages", []string{`{"tools":[` + tool("first") + `],"nextCursor":"second"}`, `{"tools":[` + tool("second") + `]}`}, "APPLIED", "SAME_ORDERED_NAMES"},
		{"empty-then-nonempty", []string{`{"tools":[],"nextCursor":"second"}`, `{"tools":[` + tool("second") + `]}`}, "APPLIED", "SAME_ORDERED_NAMES"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, p := newPeer(t, func(p *peer, index int, req peerRequest) error {
				if index >= len(tc.results) {
					return errors.New("unexpected extra request")
				}
				if index > 0 && req.Params.Cursor != "second" {
					return errors.New("original pagination cursor lost")
				}
				return p.respond(req, tc.results[index])
			})
			report, err := c.Refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if report.CollectorState != "SUCCESS" || report.Receipt == nil || report.Receipt.State != tc.state || len(report.Exchanges) != len(tc.results) {
				t.Fatalf("unexpected report %+v", report)
			}
			for range tc.results {
				select {
				case <-p.written:
				case <-p.ctx.Done():
					t.Fatal("peer write not completed")
				}
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			for index, pair := range report.Exchanges {
				if !pair.RequestWritten || !pair.ResponsePresent || !bytes.Equal(pair.Request, p.requests[index]) || !bytes.Equal(pair.Response, p.responses[index]) {
					t.Fatalf("exact original pair not bound: %+v", pair)
				}
			}
			if tc.visibility == "" {
				if len(report.PageMappings) != 0 || report.Receipt.Reason != "STAGING_FAILED" {
					t.Fatal("raw refusal hidden", report)
				}
			} else if len(report.PageMappings) != len(tc.results) || report.PageMappings[0].Result.F_reason != tc.visibility {
				t.Fatalf("Gooo correspondence lost: %+v", report)
			}
			if tc.state == "RETAINED" {
				requirePreserved(t, c, report)
			}
			if tc.name == "SDK-filtered-header" && (report.Receipt.Decision == nil || report.Receipt.Decision.Reason != "INVALIDATED") {
				t.Fatal("Gooo row mismatch did not invalidate actual candidate", report)
			}
		})
	}
}

func TestRPCPageFailureRetainsCompleteOldCatalog(t *testing.T) {
	for _, tc := range []struct {
		name        string
		code        int64
		failurePage int
	}{
		{"generic-later-page", -32001, 1},
		{"method-not-found-first-page", -32601, 0},
		{"method-not-found-later-page", -32601, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, p := newPeer(t, func(p *peer, index int, req peerRequest) error {
				if index < tc.failurePage {
					return p.respond(req, `{"tools":[`+tool("partial")+`],"nextCursor":"more"}`)
				}
				return p.rpcError(req, tc.code)
			})
			before := c.Snapshot()
			report, err := c.Refresh(context.Background())
			var rpcErr *jsonrpc.Error
			if !errors.As(err, &rpcErr) || rpcErr.Code != tc.code {
				t.Fatalf("SDK RPC error changed: got %v, want code %d", err, tc.code)
			}
			requirePreserved(t, c, report)
			if report.CollectorState != "ERROR" || report.Receipt.Reason != "STAGING_FAILED" || len(report.Exchanges) != tc.failurePage+1 {
				t.Fatalf("failed collection became complete: %+v", report)
			}
			if report.Stored.RegistryVersion != before.RegistryVersion || report.Stored.SHA256 != before.SHA256 || !bytes.Equal(report.Stored.Bytes, before.Bytes) {
				t.Fatalf("prior catalog changed: before %+v, after %+v", before, report.Stored)
			}
			for range report.Exchanges {
				select {
				case <-p.written:
				case <-p.ctx.Done():
					t.Fatal("peer write not completed")
				}
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			for index, pair := range report.Exchanges {
				if !pair.RequestWritten || !pair.ResponsePresent || !bytes.Equal(pair.Request, p.requests[index]) || !bytes.Equal(pair.Response, p.responses[index]) {
					t.Fatalf("exact original pair not bound: %+v", pair)
				}
			}
			failed := report.Exchanges[tc.failurePage]
			var request peerRequest
			if err := json.Unmarshal(failed.Request, &request); err != nil {
				t.Fatal(err)
			}
			wantCursor := ""
			if tc.failurePage > 0 {
				wantCursor = "more"
			}
			if request.Params.Cursor != wantCursor || !bytes.Contains(failed.Response, []byte(fmt.Sprintf(`"code":%d`, tc.code))) {
				t.Fatalf("failed page cursor or RPC response lost: %+v", failed)
			}
		})
	}
}

func TestMalformedListResultNeverReplacesOrPanics(t *testing.T) {
	for _, result := range []string{`null`, `{}`, `{"tools":null}`, `{"tools":3}`} {
		t.Run(result, func(t *testing.T) {
			c, _ := newPeer(t, func(p *peer, _ int, req peerRequest) error { return p.respond(req, result) })
			defer func() {
				if panicValue := recover(); panicValue != nil {
					t.Errorf("malformed tools/list result panicked: %v", panicValue)
				}
			}()
			report, _ := c.Refresh(context.Background())
			requirePreserved(t, c, report)
			if len(report.Exchanges) != 1 || !report.Exchanges[0].ResponsePresent || !bytes.Contains(report.Exchanges[0].Response, []byte(`"result" : `+result)) || len(report.PageMappings) != 0 {
				t.Fatal("malformed original response was not retained separately", report)
			}
		})
	}
}

func TestObservedNotificationDuringCollectionInvalidatesLease(t *testing.T) {
	c, _ := newPeer(t, func(p *peer, index int, req peerRequest) error {
		if index == 0 {
			return p.respond(req, `{"tools":[`+tool("first")+`],"nextCursor":"more"}`)
		}
		if err := p.notify(); err != nil {
			return err
		}
		return p.respond(req, `{"tools":[`+tool("second")+`]}`)
	})
	report, err := c.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	requirePreserved(t, c, report)
	if report.CollectorState != "SUCCESS" || report.Receipt.Decision == nil || report.Receipt.Decision.Reason != "INVALIDATED" {
		t.Fatalf("notification did not revoke captured tuple: %+v", report)
	}
}

func TestCursorCycleAndPageBoundStopCollection(t *testing.T) {
	for _, cycle := range []bool{true, false} {
		t.Run(fmt.Sprint(cycle), func(t *testing.T) {
			c, _ := newPeer(t, func(p *peer, index int, req peerRequest) error {
				next := fmt.Sprint(index + 1)
				if cycle {
					next = "repeat"
				}
				return p.respond(req, `{"tools":[],"nextCursor":"`+next+`"}`)
			})
			report, err := c.Refresh(context.Background())
			if err == nil {
				t.Fatal("unbounded pagination accepted")
			}
			requirePreserved(t, c, report)
			want := MaxPages
			if cycle {
				want = 2
			}
			if len(report.Exchanges) != want || report.CollectorState != "ERROR" {
				t.Fatal(report)
			}
		})
	}
}

func TestContextCancellationAndDeadlineDuringSecondPage(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			c, p := newPeer(t, func(p *peer, index int, req peerRequest) error {
				if index == 0 {
					return p.respond(req, `{"tools":[`+tool("partial")+`],"nextCursor":"more"}`)
				}
				close(entered)
				select {
				case <-release:
					return p.respond(req, `{"tools":[]}`)
				case <-p.ctx.Done():
					return p.ctx.Err()
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			}
			defer cancel()
			type outcome struct {
				report Report
				err    error
			}
			done := make(chan outcome, 1)
			go func() { r, e := c.Refresh(ctx); done <- outcome{r, e} }()
			select {
			case <-entered:
			case <-p.ctx.Done():
				t.Fatal("second page not requested")
			}
			if !deadline {
				cancel()
			}
			select {
			case result := <-done:
				if result.err == nil {
					t.Fatal("cancellation disappeared")
				}
				requirePreserved(t, c, result.report)
				want := "CANCELLED"
				if deadline {
					want = "TIMEOUT"
				}
				if result.report.CollectorState != want {
					t.Fatal(result.report)
				}
			case <-time.After(2 * time.Second):
				_ = c.Close()
				close(release)
				t.Fatal("Refresh did not return after context ended")
			}
			close(release)
		})
	}
}

func TestNewerAttemptWinsBeforeWaitingForIOGate(t *testing.T) {
	firstEntered, release := make(chan struct{}), make(chan struct{})
	c, _ := newPeer(t, func(p *peer, index int, req peerRequest) error {
		if index == 0 {
			close(firstEntered)
			select {
			case <-release:
			case <-p.ctx.Done():
				return p.ctx.Err()
			}
		}
		return p.respond(req, `{"tools":[`+tool(fmt.Sprint(index))+`]}`)
	})
	begun := make(chan string, 2)
	c.afterBegin = func(id string) { begun <- id }
	type outcome struct {
		report Report
		err    error
	}
	a, b := make(chan outcome, 1), make(chan outcome, 1)
	go func() { r, e := c.Refresh(context.Background()); a <- outcome{r, e} }()
	<-begun
	<-firstEntered
	go func() { r, e := c.Refresh(context.Background()); b <- outcome{r, e} }()
	<-begun
	close(release)
	first, second := <-a, <-b
	if first.err != nil || second.err != nil {
		t.Fatal(first.err, second.err)
	}
	if first.report.Receipt == nil || first.report.Receipt.State != "RETAINED" || first.report.Receipt.Decision == nil || first.report.Receipt.Decision.Reason != "SUPERSEDED_ATTEMPT" || second.report.Receipt == nil || second.report.Receipt.State != "APPLIED" {
		t.Fatal(first.report, second.report)
	}
}

func TestCancelledQueuedAttemptSupersedesWithoutClosingOwner(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	c, _ := newPeer(t, func(p *peer, index int, req peerRequest) error {
		if index == 0 {
			close(entered)
			select {
			case <-release:
			case <-p.ctx.Done():
				return p.ctx.Err()
			}
		}
		return p.respond(req, `{"tools":[`+tool(fmt.Sprint(index))+`]}`)
	})
	begun := make(chan string, 3)
	c.afterBegin = func(id string) { begun <- id }
	type outcome struct {
		report Report
		err    error
	}
	first, queued := make(chan outcome, 1), make(chan outcome, 1)
	go func() { r, e := c.Refresh(context.Background()); first <- outcome{r, e} }()
	<-begun
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { r, e := c.Refresh(ctx); queued <- outcome{r, e} }()
	<-begun
	cancel()
	second := <-queued
	if !errors.Is(second.err, context.Canceled) || second.report.CollectorState != "CANCELLED" || len(second.report.Exchanges) != 0 {
		t.Fatal("queued cancellation did not retain its own state", second)
	}
	requirePreserved(t, c, second.report)
	close(release)
	a := <-first
	if a.err != nil || a.report.Receipt == nil || a.report.Receipt.Decision == nil || a.report.Receipt.Decision.Reason != "SUPERSEDED_ATTEMPT" {
		t.Fatal("cancelled newer attempt did not supersede older collection", a)
	}
	requirePreserved(t, c, a.report)
	third, err := c.Refresh(context.Background())
	if err != nil || third.Receipt == nil || third.Receipt.State != "APPLIED" || len(third.Exchanges) != 1 {
		t.Fatal("queued cancellation closed the active owner's connection", third, err)
	}
}

func TestDifferentNegotiatedProtocolClosesWithoutCollecting(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer remote.Close()
		reader := bufio.NewReader(remote)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			done <- err
			return
		}
		var req peerRequest
		if err := json.Unmarshal(line, &req); err != nil {
			done <- err
			return
		}
		if req.Method != "initialize" {
			done <- fmt.Errorf("unexpected first method %s", req.Method)
			return
		}
		_, err = io.WriteString(remote, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"different-profile","version":"1"}}}`+"\n")
		if err != nil {
			done <- err
			return
		}
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				done <- nil
				return
			}
			var req peerRequest
			if err := json.Unmarshal(line, &req); err != nil {
				done <- err
				return
			}
			if req.Method == "tools/list" {
				done <- errors.New("collection ran under a different negotiated protocol")
				return
			}
		}
	}()
	c, err := Connect(ctx, local, []byte("prior opaque catalog"))
	if c != nil {
		_ = c.Close()
		t.Fatal("different protocol accepted")
	}
	if err == nil {
		t.Fatal("different negotiated protocol did not return an error")
	}
	select {
	case peerErr := <-done:
		if peerErr != nil {
			t.Fatal(peerErr)
		}
	case <-ctx.Done():
		t.Fatal("profile refusal did not close owned IO")
	}
}

type blockedWrite struct {
	net.Conn
	method     string
	entered    chan struct{}
	closed     chan struct{}
	once       sync.Once
	signal     sync.Once
	closeCalls atomic.Int32
}

func (w *blockedWrite) Write(b []byte) (int, error) {
	method := w.method
	if method == "" {
		method = "tools/list"
	}
	if bytes.Contains(b, []byte(`"method":"`+method+`"`)) {
		w.signal.Do(func() { close(w.entered) })
		<-w.closed
		return 0, io.ErrClosedPipe
	}
	return w.Conn.Write(b)
}
func (w *blockedWrite) Close() error {
	w.closeCalls.Add(1)
	w.once.Do(func() { close(w.closed) })
	return w.Conn.Close()
}

func TestCancellationUnblocksAnOwnedBlockedWriter(t *testing.T) {
	var wire *blockedWrite
	c, p := newPeerWithConn(t, func(p *peer, _ int, r peerRequest) error { return p.respond(r, `{"tools":[]}`) }, func(conn net.Conn) io.ReadWriteCloser {
		wire = &blockedWrite{Conn: conn, entered: make(chan struct{}), closed: make(chan struct{})}
		return wire
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		report Report
		err    error
	}
	done := make(chan outcome, 1)
	go func() { r, err := c.Refresh(ctx); done <- outcome{r, err} }()
	select {
	case <-wire.entered:
	case <-p.ctx.Done():
		t.Fatal("tools/list write was not attempted")
	}
	cancel()
	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) || result.report.CollectorState != "CANCELLED" {
			t.Fatal("cancelled write lost its cause", result.err, result.report)
		}
		requirePreserved(t, c, result.report)
	case <-time.After(time.Second):
		_ = c.Close()
		<-done
		t.Fatal("cancelled owned writer remained blocked")
	}
}

func TestCancellationUnblocksOwnedInitializationWriter(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	wire := &blockedWrite{Conn: local, method: "initialize", entered: make(chan struct{}), closed: make(chan struct{})}
	defer wire.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		client, err := Connect(ctx, wire, []byte(`{"tools":[]}`))
		if client != nil {
			_ = client.Close()
		}
		done <- err
	}()
	select {
	case <-wire.entered:
	case <-time.After(time.Second):
		cancel()
		_ = wire.Close()
		<-done
		t.Fatal("initialize write was not attempted")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("initialization lost cancellation", err)
		}
		if calls := wire.closeCalls.Load(); calls != 1 {
			t.Fatalf("failed Connect closed the underlying owner %d times", calls)
		}
	case <-time.After(time.Second):
		_ = wire.Close()
		<-done
		t.Fatal("cancelled initialization writer remained blocked")
	}
}

func TestOversizedAndDuplicateEnvelopeFramesRetain(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(fmt.Sprint(oversized), func(t *testing.T) {
			c, _ := newPeer(t, func(p *peer, _ int, req peerRequest) error {
				if oversized {
					_, err := io.WriteString(p.conn, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"result":{"tools":[],"padding":"`+strings.Repeat("x", MaxFrameBytes)+`"}}`+"\n")
					return err
				}
				_, err := io.WriteString(p.conn, `{"jsonrpc":"2.0","id":`+string(req.ID)+`,"id":`+string(req.ID)+`,"result":{"tools":[]}}`+"\n")
				return err
			})
			report, err := c.Refresh(context.Background())
			if err == nil {
				t.Fatal("invalid original frame accepted")
			}
			requirePreserved(t, c, report)
			if report.CaptureFailure == nil || len(report.CaptureFailure.Frame) > MaxFrameBytes {
				t.Fatal(report)
			}
			if oversized && report.CaptureFailure.Complete {
				t.Fatal("oversized prefix presented as a complete original frame")
			}
		})
	}
}
