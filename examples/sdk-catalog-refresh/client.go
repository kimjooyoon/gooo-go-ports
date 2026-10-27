// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.

// Package sdkcatalogrefresh collects MCP tool catalogs with Gooo rules that retain
// the previous catalog after incomplete collection or SDK tool filtering.
//
// Connect owns a newline-delimited JSON-RPC connection and a private Go MCP SDK
// v1.8.0 session. This experimental profile requires negotiated protocol
// 2025-11-25. It does not support HTTP/SSE or cached-list protocols.
//
// Refresh captures original IO pages before SDK parsing, compares their ordered
// names with the SDK-visible names, and asks the Gooo catalog rule whether to
// replace local storage. Inspect both the returned Report and error. Retained
// bytes preserve previous storage; they grant no permission to expose or invoke
// tools. No method invokes tools or writes durable storage.
//
// The rules are written in the Gooo language and compiled to the committed Go
// libraries. Importing this package requires no runtime Gooo compiler.
// See https://github.com/kimjooyoon/meta-ontology-go for the language and
// https://github.com/kimjooyoon/gooo-go-ports for rule sources and finite evidence.
package sdkcatalogrefresh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh"
	host "github.com/kimjooyoon/gooo-go-ports/examples/catalog-refresh/host"
	visibility "github.com/kimjooyoon/gooo-go-ports/examples/sdk-catalog-refresh/internal/rowvisibility"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const ProtocolVersion = "2025-11-25"
const Profile = "gooo/SDK1.8-original-IO-ordered-row-refresh/v1"
const CollectionTimeout = 30 * time.Second

type PageMapping struct {
	PageIndex      int                        `json:"page_index"`
	RequestID      string                     `json:"request_id"`
	ResponseSHA256 string                     `json:"original_response_sha256"`
	SDKNamesSHA256 string                     `json:"SDK_names_Go_JSON_sha256"`
	Result         visibility.R_RowVisibility `json:"result"`
}

type Problem struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Report struct {
	Schema         string             `json:"schema"`
	Profile        string             `json:"host_profile"`
	Protocol       string             `json:"negotiated_protocol"`
	RowRuleOrigin  json.RawMessage    `json:"row_rule_generation_origin"`
	CollectorState string             `json:"collector_state,omitempty"`
	AttemptID      string             `json:"attempt_id,omitempty"`
	Exchanges      []Exchange         `json:"exchanges"`
	CaptureFailure *CaptureFailure    `json:"capture_failure,omitempty"`
	PageMappings   []PageMapping      `json:"page_mappings"`
	Receipt        *refresh.Receipt   `json:"receipt,omitempty"`
	Stored         host.StoredCatalog `json:"stored_catalog"`
	Diagnostic     *Problem           `json:"diagnostic,omitempty"`
}

// Client owns one SDK session and one registry. Call Close to release the IO
// connection. IO Close must unblock pending reads and writes, as required by the
// underlying SDK connection contract. No method invokes tools or writes files.
type Client struct {
	sdk        *mcp.ClientSession
	wire       *capturedIO
	registry   *refresh.Registry
	gate       chan struct{}
	closeOnce  sync.Once
	closeErr   error
	afterBegin func(string) // private deterministic test observation; nil in normal use
}

// Connect takes ownership of conn, including on error. The requested protocol is
// checked after negotiation; a newer cached-list profile is not silently used.
func Connect(ctx context.Context, conn io.ReadWriteCloser, previous []byte) (*Client, error) {
	if conn == nil {
		return nil, errors.New("IO connection is required")
	}
	failed := true
	closeOwned := conn.Close
	defer func() {
		if failed {
			_ = closeOwned()
		}
	}()
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	registry, err := refresh.NewRegistry(previous)
	if err != nil {
		return nil, err
	}
	wire := newCapturedIO(conn, registry.ChangeScope, registry.Invalidate)
	closeOwned = wire.Close
	ctx, cancel := context.WithTimeout(ctx, CollectionTimeout)
	defer cancel()
	stop := wire.closeOnCancellation(ctx)
	defer stop()
	sdk := mcp.NewClient(&mcp.Implementation{Name: "gooo-sdk-catalog-refresh", Version: "experimental"},
		&mcp.ClientOptions{Capabilities: &mcp.ClientCapabilities{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	session, err := sdk.Connect(ctx, &mcp.IOTransport{Reader: wire, Writer: wire, MaxLineLength: MaxFrameBytes},
		&mcp.ClientSessionOptions{ProtocolVersion: ProtocolVersion})
	if err != nil {
		_ = wire.Close()
		return nil, errors.Join(err, ctx.Err())
	}
	if result := session.InitializeResult(); result == nil || result.ProtocolVersion != ProtocolVersion {
		_ = wire.Close()
		_ = session.Close()
		return nil, errors.New("negotiated protocol outside original-IO capture profile")
	}
	c := &Client{sdk: session, wire: wire, registry: registry, gate: make(chan struct{}, 1)}
	c.gate <- struct{}{}
	failed = false
	return c, nil
}

func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		// Revoke the scope and unblock IO before the SDK's graceful wait.
		c.closeErr = c.wire.Close()
		if err := c.sdk.Close(); c.closeErr == nil {
			c.closeErr = err
		}
	})
	return c.closeErr
}

func (c *Client) Snapshot() host.StoredCatalog { return c.registry.Snapshot() }

func hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func collectionState(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "TIMEOUT"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELLED"
	}
	return "ERROR"
}

func (c *Client) result(a *refresh.Attempt, report Report, pages []refresh.SnapshotPage, sdkNames [][]string, callErr error) (Report, error) {
	candidate, err := a.Stage(pages, report.CollectorState)
	if err != nil {
		r := c.registry.Refresh(a, pages, report.CollectorState)
		report.Receipt = &r
		if report.Diagnostic == nil {
			report.Diagnostic = &Problem{"raw_admission", "ORIGINAL_PAGES_REFUSED", err.Error()}
		}
	} else {
		if report.CollectorState == "SUCCESS" {
			facts, _ := candidate.Inputs()
			for page := range pages {
				wireNames := []string{}
				for index, pageIndex := range facts.RowPageIndexes {
					if pageIndex == int64(page) {
						wireNames = append(wireNames, facts.OriginalNames[index])
					}
				}
				actual := visibility.Evaluate(visibility.R_RowNames{F_names: wireNames}, visibility.R_RowNames{F_names: sdkNames[page]})
				namesJSON, _ := json.Marshal(sdkNames[page])
				report.PageMappings = append(report.PageMappings, PageMapping{page, report.Exchanges[page].RequestID, hash([]byte(pages[page].ResponseJSON)), hash(namesJSON), actual})
				if !actual.F_mapping_equal {
					if err := c.registry.Invalidate(); err != nil {
						_ = c.wire.Close()
						callErr = err
						report.Diagnostic = &Problem{"SDK_row_visibility", "INVALIDATION_FAILED", err.Error()}
					}
				}
			}
		}
		r := c.registry.Commit(candidate)
		report.Receipt = &r
	}
	report.Stored = c.registry.Snapshot()
	return report, callErr
}

// Refresh captures the lease before waiting for the wire gate: a newer request
// supersedes an older one even if the newer request is subsequently cancelled.
// Cancellation during active IO closes the owned connection to unblock writers;
// create a new Client to reconnect. Retained bytes never grant call authority.
func (c *Client) Refresh(ctx context.Context) (Report, error) {
	report := Report{Schema: "gooo/SDK-original-catalog-refresh-report/v1", Profile: Profile, Protocol: ProtocolVersion,
		RowRuleOrigin: json.RawMessage(visibility.GoooGenerationOrigin),
		Exchanges:     []Exchange{}, PageMappings: []PageMapping{}, Stored: c.registry.Snapshot()}
	if ctx == nil {
		return report, errors.New("context is required")
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	ctx, cancel := context.WithTimeout(ctx, CollectionTimeout)
	defer cancel()
	attempt, err := c.registry.Begin(ctx)
	if err != nil {
		return report, err
	}
	report.AttemptID = attempt.ID()
	if c.afterBegin != nil {
		c.afterBegin(attempt.ID())
	}
	select {
	case <-ctx.Done():
		report.CollectorState = collectionState(ctx.Err())
		return c.result(attempt, report, nil, nil, ctx.Err())
	case <-c.wire.done:
		report.CollectorState = "ERROR"
		return c.result(attempt, report, nil, nil, errors.New("owned connection is closed"))
	case <-c.gate:
	}
	defer func() { c.gate <- struct{}{} }()
	if err := c.wire.begin(); err != nil {
		report.CollectorState = "ERROR"
		return c.result(attempt, report, nil, nil, err)
	}
	stop := c.wire.closeOnCancellation(ctx)
	defer stop()
	pages := []refresh.SnapshotPage{}
	names := [][]string{}
	cursor := ""
	seen := map[string]bool{"": true}
	report.CollectorState = "SUCCESS"
	var callErr error
	for page := 0; page < MaxPages; page++ {
		listed, err := c.sdk.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			callErr = errors.Join(err, ctx.Err())
			report.CollectorState = collectionState(callErr)
			report.Diagnostic = &Problem{"collect", "SDK_LIST_FAILED", err.Error()}
			if ctx.Err() != nil {
				_ = c.wire.Close()
			}
			break
		}
		pair, err := c.wire.latest(page)
		if err != nil {
			callErr = err
			report.CollectorState = "ERROR"
			report.Diagnostic = &Problem{"capture", "NO_FRESH_PAIR", err.Error()}
			break
		}
		pages = append(pages, refresh.SnapshotPage{RequestJSON: string(pair.Request), ResponseJSON: string(pair.Response)})
		pageNames := []string{}
		for _, tool := range listed.Tools {
			if tool == nil {
				callErr = errors.New("nil tool survived SDK row filtering")
				break
			}
			pageNames = append(pageNames, tool.Name)
		}
		names = append(names, pageNames)
		if callErr != nil {
			report.CollectorState = "ERROR"
			break
		}
		next := listed.NextCursor
		if next == "" {
			break
		}
		if len(next) > 1024 || seen[next] || page == MaxPages-1 {
			callErr = errors.New("cursor cycle, cursor bound or page bound reached")
			report.CollectorState = "ERROR"
			report.Diagnostic = &Problem{"pagination", "INCOMPLETE_BOUNDED_COLLECTION", callErr.Error()}
			break
		}
		seen[next] = true
		cursor = next
	}
	exchanges, failure, closed := c.wire.finish()
	report.Exchanges, report.CaptureFailure = exchanges, failure
	if failure != nil || closed {
		if callErr == nil {
			callErr = fmt.Errorf("original IO capture failed or closed")
		}
		if report.CollectorState == "SUCCESS" {
			report.CollectorState = "ERROR"
		}
	}
	// Preserve a complete failure response as original data, without converting
	// it into a successful empty list. Incomplete pairs remain in Exchanges.
	for len(pages) < len(exchanges) && exchanges[len(pages)].ResponsePresent {
		pair := exchanges[len(pages)]
		pages = append(pages, refresh.SnapshotPage{RequestJSON: string(pair.Request), ResponseJSON: string(pair.Response)})
	}
	return c.result(attempt, report, pages, names, callErr)
}
