package main

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Shapes follow compiler205's CompositionTrace, CompositionBridgeTrace/Window,
// CompositionFieldWrite/Use, CompositionLocalWrite/Use and condition records.
// Their exact source hashes are recorded with this migration's review evidence.
// These optional diagnostics are checked for finite typed shape, not lineage,
// completeness or authority. The existing delivery/source/oracle checks remain
// responsible for the result. Collection/call diagnostic payloads are outside
// this reader and unknown fields fail closed.
const traceMetadataLimit = 65536

func inspectTraceMetadata(trace map[string]any) error {
	if _, ok := trace["case_index"]; !ok {
		return fmt.Errorf("trace case index missing")
	}
	if _, ok := trace["deliveries"]; !ok {
		return fmt.Errorf("trace deliveries missing")
	}
	for name, value := range trace {
		switch name {
		case "case_index", "deliveries":
			// Exact index, one delivery and its complete contents are checked
			// by the existing caller immediately after this shape check.
		case "conditions_observed", "reads_observed":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("trace diagnostic Boolean: %s", name)
			}
		case "field_frames", "field_frame_dropped", "field_write_dropped", "field_use_dropped",
			"conditions_dropped", "local_frames", "local_frame_dropped", "local_write_dropped",
			"local_use_dropped", "reads_dropped", "read_registration_dropped":
			number, ok := traceMetadataInteger(value)
			if !ok || number < 0 || number > traceMetadataLimit {
				return fmt.Errorf("trace diagnostic finite count: %s", name)
			}
		case "field_local_bridge":
			if err := inspectTraceBridge(value); err != nil {
				return err
			}
		case "field_writes", "field_uses", "conditions", "local_writes", "local_uses":
			rows, ok := traceMetadataList(value)
			if !ok {
				return fmt.Errorf("trace diagnostic array: %s", name)
			}
			for _, row := range rows {
				var integers, booleans, optionalText []string
				switch name {
				case "field_writes":
					integers = []string{"site_index", "frame", "use_start", "use_end", "uses_dropped", "parent_assignment", "completion"}
					booleans = []string{"completed"}
				case "field_uses":
					integers = []string{"site_index", "frame", "assignment", "completion_clock"}
					optionalText = []string{"reason"}
				case "conditions":
					integers = []string{"site_index"}
					booleans = []string{"value"}
				case "local_writes":
					integers = []string{"site_index", "frame", "read_start", "read_end", "use_start", "use_end", "reads_dropped", "uses_dropped"}
					booleans = []string{"completed"}
				case "local_uses":
					integers = []string{"site_index", "frame", "assignment"}
					optionalText = []string{"reason"}
				}
				if !traceMetadataRecord(row, integers, booleans, optionalText) {
					return fmt.Errorf("trace diagnostic record: %s", name)
				}
			}
		default:
			return fmt.Errorf("unsupported trace field: %s", name)
		}
	}
	return nil
}

func inspectTraceBridge(value any) error {
	if value == nil {
		return nil
	}
	bridge, ok := value.(map[string]any)
	if !ok || !keys(bridge, "locals", "fields", "local_clocks", "field_clocks", "completions") {
		return fmt.Errorf("trace bridge exact record")
	}
	if count, ok := traceMetadataInteger(bridge["completions"]); !ok || count < 0 || count > traceMetadataLimit {
		return fmt.Errorf("trace bridge completion count")
	}
	for _, name := range []string{"locals", "fields", "local_clocks", "field_clocks"} {
		rows, ok := traceMetadataList(bridge[name])
		if !ok {
			return fmt.Errorf("trace bridge finite array")
		}
		for _, row := range rows {
			if name == "locals" || name == "fields" {
				if !traceMetadataRecord(row, []string{"other_start", "other_end", "other_drops", "start_clock", "completion"}, nil, nil) {
					return fmt.Errorf("trace bridge window")
				}
			} else if _, ok := traceMetadataInteger(row); !ok {
				return fmt.Errorf("trace bridge clock")
			}
		}
	}
	return nil
}

func traceMetadataList(value any) ([]any, bool) {
	if value == nil {
		return nil, true
	}
	rows, ok := value.([]any)
	return rows, ok && len(rows) <= traceMetadataLimit
}

func traceMetadataInteger(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	integer, err := number.Int64()
	return integer, err == nil
}

func traceMetadataRecord(value any, integers, booleans, optionalText []string) bool {
	record, ok := value.(map[string]any)
	if !ok {
		return false
	}
	known := map[string]bool{}
	for _, name := range integers {
		known[name] = true
		if _, ok := traceMetadataInteger(record[name]); !ok {
			return false
		}
	}
	for _, name := range booleans {
		known[name] = true
		if _, ok := record[name].(bool); !ok {
			return false
		}
	}
	for _, name := range optionalText {
		known[name] = true
		if value, present := record[name]; present {
			text, ok := value.(string)
			if !ok || !utf8.ValidString(text) || len(text) > 1024 {
				return false
			}
		}
	}
	for name := range record {
		if !known[name] {
			return false
		}
	}
	return true
}
