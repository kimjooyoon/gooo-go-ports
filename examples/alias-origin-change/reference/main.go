// Copyright (c) 2026 Gooo Go Ports contributors. BSD-3-Clause; see ../LICENSE.
package main

import (
	"encoding/json"
	"os"
)

const entry = "gooo/tool-origin:CompareAliasOrigin"

func strings(x any) []string {
	a := x.([]any)
	v := make([]string, len(a))
	for i, s := range a {
		v[i] = s.(string)
	}
	return v
}
func assess(in map[string]any) map[string]any {
	out := map[string]any{"context_id": in["context_id"], "state": "ABSENT", "reason": "ALIAS_NOT_IN_SUPPLIED_CATALOGS", "before_index": -1, "after_index": -1, "conflict_catalog": "", "conflict_first_index": -1, "conflict_second_index": -1, "execution_authorized": false}
	target := in["target_alias"].(string)
	invalid := target == "" || in["context_id"].(string) == ""
	cols := map[string][]string{}
	for _, side := range []string{"before", "after"} {
		for _, col := range []string{"servers", "names", "aliases"} {
			k := side + "_" + col
			cols[k] = strings(in[k])
		}
		n := len(cols[side+"_aliases"])
		if len(cols[side+"_servers"]) != n || len(cols[side+"_names"]) != n {
			invalid = true
		}
	}
	if !invalid {
		for _, v := range cols {
			for _, s := range v {
				if s == "" {
					invalid = true
				}
			}
		}
	}
	if invalid {
		out["state"] = "INVALID_INPUT"
		out["reason"] = "INVALID_CATALOG_COLUMNS"
		return out
	}
	for _, side := range []string{"before", "after"} {
		seen := map[string]int{}
		for index, alias := range cols[side+"_aliases"] {
			if prior, found := seen[alias]; found {
				if out["conflict_catalog"] == "" {
					if side == "before" {
						out["conflict_catalog"] = "BEFORE"
					} else {
						out["conflict_catalog"] = "AFTER"
					}
					out["conflict_first_index"] = prior
					out["conflict_second_index"] = index
				}
			} else {
				seen[alias] = index
			}
			if alias == target {
				out[side+"_index"] = index
			}
		}
	}
	if out["conflict_catalog"] != "" {
		out["state"] = "AMBIGUOUS"
		out["reason"] = "DUPLICATE_EXPOSED_NAME"
		out["before_index"] = -1
		out["after_index"] = -1
		return out
	}
	b, a := out["before_index"].(int), out["after_index"].(int)
	switch {
	case b >= 0 && a >= 0:
		if cols["before_servers"][b] == cols["after_servers"][a] && cols["before_names"][b] == cols["after_names"][a] {
			out["state"] = "UNCHANGED"
			out["reason"] = "SAME_DECLARED_ORIGIN"
		} else {
			out["state"] = "ORIGIN_CHANGED"
			out["reason"] = "DIFFERENT_DECLARED_ORIGIN"
		}
	case b >= 0:
		out["state"] = "REMOVED"
		out["reason"] = "ALIAS_REMOVED_FROM_SUPPLIED_CATALOG"
	case a >= 0:
		out["state"] = "ADDED"
		out["reason"] = "ALIAS_ADDED_TO_SUPPLIED_CATALOG"
	}
	return out
}
func main() {
	var doc map[string]any
	if err := json.NewDecoder(os.Stdin).Decode(&doc); err != nil {
		panic(err)
	}
	for _, row := range doc["cases"].([]any) {
		c := row.(map[string]any)
		in := c["inputs"].(map[string]any)[entry].(map[string]any)
		c["expected"] = map[string]any{entry: assess(in)}
	}
	if err := json.NewEncoder(os.Stdout).Encode(doc); err != nil {
		panic(err)
	}
}
