//go:build js && wasm

package main

import (
	"errors"
	"math"
	"strconv"
	"syscall/js"
)

// Only dense ordinary arrays with their index properties and length are
// accepted. Missing elements, extra own fields and typed arrays are not coerced.
func arrayLength(value js.Value, limit int) (int, error) {
	if value.Type() != js.TypeObject || !js.Global().Get("Array").Call("isArray", value).Bool() {
		return 0, errors.New("expected an exact array")
	}
	n := value.Length()
	if n > limit {
		return 0, errors.New("array exceeds the transport bound")
	}
	keys := js.Global().Get("Reflect").Call("ownKeys", value)
	if keys.Length() != n+1 {
		return 0, errors.New("array contains missing or extra fields")
	}
	for i := 0; i < n; i++ {
		if !js.Global().Get("Object").Call("hasOwn", value, strconv.Itoa(i)).Bool() {
			return 0, errors.New("array contains a missing field")
		}
	}
	return n, nil
}

func browserFields(mode string, value js.Value) ([][]uint16, error) {
	want := fieldCount(mode)
	if want == 0 {
		return nil, errors.New("unknown mode")
	}
	n, err := arrayLength(value, 4)
	if err != nil || n != want {
		return nil, errors.New("incorrect field array")
	}
	fields := make([][]uint16, n)
	for i := range fields {
		field := value.Index(i)
		count, err := arrayLength(field, maxFieldUTF16Units)
		if err != nil {
			return nil, err
		}
		fields[i] = make([]uint16, count)
		for j := range fields[i] {
			unit := field.Index(j)
			if unit.Type() != js.TypeNumber {
				return nil, errors.New("UTF-16 code units must be integers")
			}
			value := unit.Float()
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 65535 || math.Trunc(value) != value {
				return nil, errors.New("UTF-16 code unit is outside 0..65535")
			}
			fields[i][j] = uint16(value)
		}
	}
	return fields, nil
}

func main() {
	evaluate := js.FuncOf(func(_ js.Value, args []js.Value) (result any) {
		defer func() {
			if recover() != nil {
				result = failureJSON("browser transport or checked execution failed")
			}
		}()
		if len(args) != 2 || args[0].Type() != js.TypeString {
			return failureJSON("expected a mode string and an exact field array")
		}
		mode := args[0].String()
		fields, err := browserFields(mode, args[1])
		if err != nil {
			return failureJSON(err.Error())
		}
		return evaluateJSON(mode, fields)
	})
	bindings := js.FuncOf(func(_ js.Value, args []js.Value) (result any) {
		defer func() {
			if recover() != nil {
				result = failureJSON("native source binding verification failed")
			}
		}()
		if len(args) != 0 {
			return failureJSON("source bindings take no arguments")
		}
		return bindingsJSON()
	})
	js.Global().Set("goooServicePolicyEvaluate", evaluate)
	js.Global().Set("goooServicePolicyBindings", bindings)
	js.Global().Set("goooServicePolicyReady", true)
	select {}
}
