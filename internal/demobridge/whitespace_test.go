package demobridge

import (
	"strconv"
	"testing"
)

func TestWhitespaceBridgePreservesRuneAndUsesOnlyCompiledBoolean(t *testing.T) {
	for _, n := range []int64{-2147483648, -1, 0, 9, 133, 160, 0x200b, 0xd800, 0xdfff, 0xfeff, 0x1f600, 0x10ffff, 0x110000, 2147483647} {
		for _, flag := range []bool{false, true} {
			input := strconv.FormatInt(n, 10)
			calls := 0
			r := readResponse(t, Run("unicode.IsSpace", []string{input, "0"}, func(op string, a, b, c, d int64) any {
				calls++
				if op != "unicode.IsSpace" || a != n || b != 0 || c != 0 || d != 0 {
					t.Fatal("rune input or unused padding changed")
				}
				return testReply{Flag: flag}
			}))
			if !r.OK || calls != 1 || r.Output == nil || r.Output.Type != "boolean" || r.Output.Value != flag || len(r.Inputs) != 2 || r.Inputs[0] != input || r.Inputs[1] != "0" {
				t.Fatalf("bridge calculated or changed the reply: %+v", r)
			}
		}
	}
}

func TestWhitespaceBridgeRejectsOutsideRuneSyntaxAndArityBeforeDispatch(t *testing.T) {
	for _, value := range []string{"-2147483649", "2147483648", "-9223372036854775808", "9223372036854775807", "-9007199254740993", "4294967328", "", " ", "\t", "\r\n", "+1", "01", "-0", "1.0", "1e2", "0x20", "NaN", "１２", "9223372036854775808"} {
		rejectBeforeCompute(t, "unicode.IsSpace", []string{value, "0"})
	}
	for _, value := range []string{"1", "-1", "00", "-0", "+0", "", " ", "2147483648"} {
		rejectBeforeCompute(t, "unicode.IsSpace", []string{"160", value})
	}
	for _, inputs := range [][]string{nil, {}, {"160"}, {"160", "0", "0"}, {"160", "0", "0", "0"}, {"160", "0", "0", "0", "0"}} {
		rejectBeforeCompute(t, "unicode.IsSpace", inputs)
	}
}
