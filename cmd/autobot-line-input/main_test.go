package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func facts() []any {
	return []any{"crm:send", false, "", true, false, "", false, false, false}
}

func encode(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOriginalValuesAndInputOnlyEnvelope(t *testing.T) {
	rows := [][]any{facts(), facts(), facts(), facts(), facts()}
	rows[0][0], rows[0][4], rows[0][5] = "", true, ""
	rows[1][0], rows[1][1], rows[1][2] = "\x00\t\r\n가😀<&>\u2028\u2029", true, "autonomous"
	rows[2][1], rows[2][2], rows[2][4], rows[2][5] = true, "draft_then_send", true, " category 'quoted' \\\u0000 "
	rows[3][1], rows[3][2], rows[3][6], rows[3][7], rows[3][8] = true, "never", true, true, true
	rows[4][0], rows[4][4], rows[4][5] = strings.Repeat("é", 512), true, strings.Repeat("x", 1024)
	var out bytes.Buffer
	if err := run(nil, bytes.NewReader(encode(t, rows)), &out); err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &document); err != nil || len(document) != 2 {
		t.Fatalf("input-only document: %v %s", err, out.Bytes())
	}
	if string(document["schema"]) != `"gooo/body-composition-inputs/v1"` || document["inputs"] == nil || document["expected"] != nil || document["cases"] != nil {
		t.Fatalf("unexpected envelope: %s", out.Bytes())
	}
	var got []map[string]any
	if err := json.Unmarshal(document["inputs"], &got); err != nil || len(got) != len(rows) {
		t.Fatalf("rows: %v", err)
	}
	for row, original := range rows {
		if len(got[row]) != 9 {
			t.Fatal("wrong input count")
		}
		for port, want := range original {
			key := "gooo/autobot/ai-lines:ResolveLine.input" + fmt.Sprint(port)
			if got[row][key] != want {
				t.Fatalf("row%d input%d changed: %#v != %#v", row, port, got[row][key], want)
			}
		}
	}
	if out.Bytes()[out.Len()-1] != '\n' {
		t.Fatal("missing final newline")
	}
}

func TestScalarEscapesRemainExact(t *testing.T) {
	for _, request := range []string{
		`[["\\ud800",false,"",false,true,"\ud83d\ude00",false,false,false]]`,
		`[["\ufffd",false,"",false,true,"\"\\\t\r\n\u0000",false,false,false]]`,
	} {
		var out bytes.Buffer
		if err := run(nil, strings.NewReader(request), &out); err != nil {
			t.Fatal(err)
		}
		var original [][]any
		var got struct {
			Inputs []map[string]any `json:"inputs"`
		}
		if json.Unmarshal([]byte(request), &original) != nil || json.Unmarshal(out.Bytes(), &got) != nil {
			t.Fatal("decode")
		}
		if got.Inputs[0][entry+".input0"] != original[0][0] || got.Inputs[0][entry+".input5"] != original[0][5] {
			t.Fatal("escaped scalar was changed")
		}
	}
}

func TestMalformedDocumentsProduceNoOutput(t *testing.T) {
	requests := []string{
		"", " \n\t", `[]`, `null`, `true`, `1`, `"x"`, `{}`, `{"x":1,"x":2}`,
		`[null]`, `[{}]`, `[{"x":1,"\u0078":2}]`, `[true]`, `[1]`, `["x"]`, `[[]]`,
		`[["x",false,"",false,false,"",false,false]]`,
		`[["x",false,"",false,false,"",false,false,false,0]]`,
		`[["x",false,"",false,false,"",false,false,false]] {}`,
		`[["x",false,"",false,false,"",false,false,false],]`,
		`[["\ud800",false,"",false,false,"",false,false,false]]`,
		`[["\udc00",false,"",false,false,"",false,false,false]]`,
		`[["\ud800x\udc00",false,"",false,false,"",false,false,false]]`,
		`[["\ud800\ud800",false,"",false,false,"",false,false,false]]`,
		`[["\ud800\\udc00",false,"",false,false,"",false,false,false]]`,
	}
	for _, request := range requests {
		t.Run(fmt.Sprint(len(request), ":", request), func(t *testing.T) {
			mustRefuse(t, []byte(request))
		})
	}
	badUTF8 := append([]byte(`[["`), 0xff)
	badUTF8 = append(badUTF8, []byte(`",false,"",false,false,"",false,false,false]]`)...)
	mustRefuse(t, badUTF8)
}

func TestEveryPortHasStrictTypeAndLateFailureWritesNothing(t *testing.T) {
	for port := range facts() {
		bad := []any{nil, []any{}, map[string]any{"same": 1}, 0, 1, json.Number("1e0")}
		if port == 0 || port == 2 || port == 5 {
			bad = append(bad, false, true)
		} else {
			bad = append(bad, "false", "true", "")
		}
		for index, value := range bad {
			t.Run(fmt.Sprintf("port%d/type%d", port, index), func(t *testing.T) {
				row := facts()
				row[port] = value
				mustRefuse(t, encode(t, [][]any{facts(), row}))
			})
		}
	}
}

func TestPresenceAndStoredEnumAreValidatedBeforeAnyDecision(t *testing.T) {
	for _, stored := range []string{"", "NEVER", "never ", " autonomous", "other", "가"} {
		row := facts()
		row[1], row[2], row[7], row[8] = true, stored, true, true
		mustRefuse(t, encode(t, [][]any{facts(), row}))
	}
	for _, port := range []int{2, 5} {
		row := facts()
		row[port], row[7], row[8] = "not absent", true, true
		mustRefuse(t, encode(t, [][]any{row}))
	}
}

func TestRawTextAndRowBoundaries(t *testing.T) {
	valid := encode(t, [][]any{facts()})
	var out bytes.Buffer
	exactRaw := append(append([]byte(nil), valid...), bytes.Repeat([]byte{' '}, maxRawBytes-len(valid))...)
	if err := run(nil, bytes.NewReader(exactRaw), &out); err != nil {
		t.Fatal("exact raw bound:", err)
	}
	mustRefuse(t, append(exactRaw, ' '))
	for _, text := range []string{strings.Repeat("x", 1025), strings.Repeat("é", 512) + "x"} {
		for _, port := range []int{0, 5} {
			row := facts()
			row[4], row[port] = true, text
			mustRefuse(t, encode(t, [][]any{row}))
		}
	}
	for _, count := range []int{128, 129} {
		rows := make([][]any, count)
		for i := range rows {
			rows[i] = facts()
		}
		var out bytes.Buffer
		err := run(nil, bytes.NewReader(encode(t, rows)), &out)
		if err == nil || out.Len() != 0 {
			t.Fatal("oversized row/wire document accepted")
		}
		want := "32768"
		if count == 129 {
			want = "128"
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("wrong boundary: %v", err)
		}
	}
}

// Build a boundary fixture independently from prepare. JSON key overhead is
// part of the pinned32KiB limit, and the emitted newline counts as one byte.
func expectedWire(t *testing.T, rows [][]any) []byte {
	t.Helper()
	inputs := make([]map[string]any, len(rows))
	for i, row := range rows {
		inputs[i] = make(map[string]any)
		for j, value := range row {
			inputs[i]["gooo/autobot/ai-lines:ResolveLine.input"+fmt.Sprint(j)] = value
		}
	}
	return append(encode(t, struct {
		Schema string           `json:"schema"`
		Inputs []map[string]any `json:"inputs"`
	}{"gooo/body-composition-inputs/v1", inputs}), '\n')
}

func TestFinalWireBoundIncludesNewlineAndJSONExpansion(t *testing.T) {
	rows := make([][]any, 20)
	for i := range rows {
		rows[i] = facts()
		rows[i][0], rows[i][4] = "", true
	}
	remaining := maxWireBytes - len(expectedWire(t, rows))
	if remaining <= 0 {
		t.Fatal("boundary fixture has no room")
	}
	lastRow, lastPort := -1, -1
	for i := range rows {
		for _, port := range []int{0, 5} {
			n := min(remaining, 1024)
			rows[i][port] = strings.Repeat("x", n)
			remaining -= n
			if n < 1024 {
				lastRow, lastPort = i, port
			}
		}
	}
	if remaining != 0 || lastRow < 0 {
		t.Fatal("fixture cannot fill exact bound")
	}
	var out bytes.Buffer
	if err := run(nil, bytes.NewReader(encode(t, rows)), &out); err != nil || out.Len() != maxWireBytes || !bytes.Equal(out.Bytes(), expectedWire(t, rows)) {
		t.Fatalf("exact final wire cap: err=%v size=%d", err, out.Len())
	}
	rows[lastRow][lastPort] = rows[lastRow][lastPort].(string) + "x"
	mustRefuse(t, encode(t, rows))
	for _, text := range []string{strings.Repeat("<", 1024), strings.Repeat("\u2028", 341), strings.Repeat("\u2029", 341), strings.Repeat("\x00", 1024)} {
		expanded := make([][]any, 8)
		for i := range expanded {
			expanded[i] = facts()
			expanded[i][0], expanded[i][4], expanded[i][5] = text, true, text
		}
		if len(expectedWire(t, expanded)) <= maxWireBytes {
			t.Fatal("expansion fixture is not oversized")
		}
		mustRefuse(t, encode(t, expanded))
	}
}

func mustRefuse(t *testing.T, raw []byte) {
	t.Helper()
	var out bytes.Buffer
	if err := run(nil, bytes.NewReader(raw), &out); err == nil || out.Len() != 0 {
		t.Fatalf("refusal emitted data: err=%v output=%d", err, out.Len())
	}
}

type readFailure struct{ err error }

func (r readFailure) Read([]byte) (int, error) { return 0, r.err }

type writeFailure struct{ err error }

func (w writeFailure) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestArgumentsAndIOFailures(t *testing.T) {
	failure := errors.New("test I/O failure")
	var out bytes.Buffer
	if err := run([]string{"--help"}, readFailure{failure}, &out); err == nil || out.Len() != 0 || errors.Is(err, failure) {
		t.Fatal("arguments were not rejected before reading")
	}
	if err := run(nil, readFailure{failure}, &out); !errors.Is(err, failure) || out.Len() != 0 {
		t.Fatalf("read failure: %v", err)
	}
	if err := run(nil, nil, &out); err == nil || out.Len() != 0 {
		t.Fatal("nil input accepted")
	}
	valid := encode(t, [][]any{facts()})
	if err := run(nil, bytes.NewReader(valid), nil); err == nil {
		t.Fatal("nil writer accepted")
	}
	if err := run(nil, bytes.NewReader(valid), writeFailure{failure}); !errors.Is(err, failure) {
		t.Fatalf("write failure: %v", err)
	}
	if err := run(nil, bytes.NewReader(valid), shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
}
