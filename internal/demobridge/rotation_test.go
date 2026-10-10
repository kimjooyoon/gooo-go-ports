package demobridge

import (
	"strconv"
	"testing"
)

func TestRotationKeepsBothDomainsAndExactInt64(t *testing.T) {
	counts := []int64{-9223372036854775808, -9223372036854775807, -9007199254740993, -9007199254740992, -9, -8, -1, 0, 1, 7, 8, 9, 9007199254740992, 9007199254740993, 9223372036854775806, 9223372036854775807}
	for x := int64(0); x <= 255; x++ {
		for _, k := range counts {
			inputs := []string{strconv.FormatInt(x, 10), strconv.FormatInt(k, 10)}
			calls := 0
			r := readResponse(t, Run("bits.RotateLeft8", inputs, func(name string, a, b, c, d int64) any {
				calls++
				if name != "bits.RotateLeft8" || a != x || b != k || c != 0 || d != 0 {
					t.Fatalf("rotation input changed: %s %d %d %d %d", name, a, b, c, d)
				}
				// Deliberately unrelated to rotation: the bridge must only forward this value.
				return testReply{First: 217}
			}))
			if !r.OK || calls != 1 || r.Inputs[0] != inputs[0] || r.Inputs[1] != inputs[1] || r.Output.Type != "integer" || r.Output.Value != "217" {
				t.Fatalf("rotation bridge altered input or output: %+v", r)
			}
		}
	}
}

func TestRotationRejectsBeforeComputation(t *testing.T) {
	for _, raw := range []string{"-1", "256", "-256", "257", "9007199254740993", "-9223372036854775808", "9223372036854775807"} {
		rejectBeforeCompute(t, "bits.RotateLeft8", []string{raw, "-1"})
	}
	for i := 0; i < 2; i++ {
		for _, raw := range []string{"", " 1", "1 ", "+1", "01", "-0", "1.0", "1e2", "0xff", "NaN", "１２", "9223372036854775808", "-9223372036854775809"} {
			inputs := []string{"129", "1"}
			inputs[i] = raw
			rejectBeforeCompute(t, "bits.RotateLeft8", inputs)
		}
	}
	for n := 0; n <= 5; n++ {
		if n == 2 {
			continue
		}
		inputs := make([]string, n)
		for i := range inputs {
			inputs[i] = "0"
		}
		rejectBeforeCompute(t, "bits.RotateLeft8", inputs)
	}
}

func TestRotationRejectsOutOfByteResult(t *testing.T) {
	for _, value := range []int64{-1, 256, -9223372036854775808, 9223372036854775807} {
		r := readResponse(t, Run("bits.RotateLeft8", []string{"1", "0"}, func(string, int64, int64, int64, int64) any { return testReply{First: value} }))
		if r.OK || r.Error == "" || r.Output != nil {
			t.Fatalf("accepted invalid rotation output: %+v", r)
		}
	}
}
