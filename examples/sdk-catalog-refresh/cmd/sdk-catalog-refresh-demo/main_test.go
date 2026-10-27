// Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	refresh "github.com/kimjooyoon/gooo-go-ports/examples/sdk-catalog-refresh"
)

func TestDemoUsesActualSDKAndPreservesFilteredCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var output bytes.Buffer
	if err := run(ctx, &output); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("expected three reports: %d", len(lines))
	}
	var previous []byte
	for index, line := range lines {
		var result struct {
			Case   string         `json:"case"`
			Report refresh.Report `json:"report"`
		}
		if err := json.Unmarshal(line, &result); err != nil {
			t.Fatal(err)
		}
		r := result.Report
		want := []string{"APPLIED", "RETAINED", "APPLIED"}[index]
		if r.CollectorState != "SUCCESS" || r.Receipt == nil || r.Receipt.State != want || len(r.Exchanges) != 1 || len(r.PageMappings) != 1 || !json.Valid(r.RowRuleOrigin) {
			t.Fatal(result)
		}
		if index == 1 && (r.PageMappings[0].Result.F_reason != "SDK_ROW_MISSING" || !bytes.Equal(previous, r.Stored.Bytes)) {
			t.Fatal("SDK filtering erased the previous catalog", result)
		}
		if index == 2 && bytes.Equal(previous, r.Stored.Bytes) {
			t.Fatal("actual complete-empty catalog did not replace previous bytes")
		}
		previous = bytes.Clone(r.Stored.Bytes)
	}
}
