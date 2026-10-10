package demobridge

import (
	"strconv"
	"testing"
)

func TestClassificationBridgePreservesOriginalRuneAndCompiledBoolean(t *testing.T) {
	for _, operation := range []string{"unicode.IsLetter", "unicode.IsDigit"} {
		for _, n := range []int64{-2147483648, -65471, -1, 0, 'A', '_', '0', 0xaa, 0xab, 0x2ec, 0x2ed, 0x301, 0x660, 0xd800, 0xdfff, 0x10400, 0x1d7d8, 0x10ffff, 0x110000, 2147483647} {
			for _, flag := range []bool{false, true} {
				input, calls := strconv.FormatInt(n, 10), 0
				r := readResponse(t, Run(operation, []string{input, "0"}, func(op string, a, b, c, d int64) any {
					calls++
					if op != operation || a != n || b != 0 || c != 0 || d != 0 {
						t.Fatal("original rune or padding changed")
					}
					return testReply{Flag: flag}
				}))
				if !r.OK || calls != 1 || r.Output == nil || r.Output.Type != "boolean" || r.Output.Value != flag || len(r.Inputs) != 2 || r.Inputs[0] != input || r.Inputs[1] != "0" {
					t.Fatalf("bridge calculated or changed the compiled reply: %+v", r)
				}
			}
		}
	}
}

func TestClassificationBridgeRejectsNarrowingAliasesAndMalformedProtocol(t *testing.T) {
	for _, operation := range []string{"unicode.IsLetter", "unicode.IsDigit"} {
		for _, raw := range []string{"-2147483649", "2147483648", "4294967361", "4294967344", "-4294967231", "-4294967248", "-9223372036854775808", "9223372036854775807", "-9007199254740993", "9007199254740993", "", " ", "\t", "\r\n", "+1", "01", "-0", "1.0", "1e2", "0x41", "NaN", "１２", "9223372036854775808", "-9223372036854775809"} {
			rejectBeforeCompute(t, operation, []string{raw, "0"})
		}
		for _, raw := range []string{"1", "-1", "00", "-0", "+0", "", " ", "2147483648"} {
			rejectBeforeCompute(t, operation, []string{"65", raw})
		}
		for _, inputs := range [][]string{nil, {}, {"65"}, {"65", "0", "0"}, {"65", "0", "0", "0"}, {"65", "0", "0", "0", "0"}} {
			rejectBeforeCompute(t, operation, inputs)
		}
	}
}
