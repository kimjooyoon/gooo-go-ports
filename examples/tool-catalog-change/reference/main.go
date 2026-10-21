// Extracted MIT-licensed gogcli selection kernel; see upstream/after/LICENSE.txt.
// Minimal metadata/flags/selected-policy fixture types; no service invocation.
package main

import (
	"encoding/json"
	"os"
	"strings"
)

type mcpToolSpec struct {
	Name       string
	Service    string
	Risk       string
	Capability string
}
type RootFlags struct{ ReadOnly bool }
type MCPPolicy struct {
	AllowWrite       bool
	AllowGmailSend   bool
	AllowGmailDelete bool
	AllowTools       []string
}

const literalAll = "all"
const mcpRiskRead = "read"
const mcpCapabilityGmailSend = "gmail_send"
const mcpCapabilityGmailDelete = "gmail_delete"

var catalog []mcpToolSpec

func mcpAllTools() []mcpToolSpec { return catalog }
func mcpToolAllowed(tool mcpToolSpec, allow []string) bool {
	for _, pattern := range allow {
		switch pattern {
		case "*", literalAll, string(tool.Risk), tool.Name, tool.Service:
			return true
		}
		if strings.HasSuffix(pattern, ".*") && strings.TrimSuffix(pattern, ".*") == tool.Service {
			return true
		}
	}
	return false
}
func mcpFilterTools(policy MCPPolicy, runtimeAllow []string, flags *RootFlags) []mcpToolSpec {
	if flags != nil && flags.ReadOnly {
		policy.AllowWrite = false
	}
	tools := make([]mcpToolSpec, 0, len(mcpAllTools()))
	for _, tool := range mcpAllTools() {
		if tool.Risk != mcpRiskRead && !policy.AllowWrite {
			continue
		}
		switch tool.Capability {
		case "":
		case mcpCapabilityGmailSend:
			if !policy.AllowWrite || !policy.AllowGmailSend {
				continue
			}
		case mcpCapabilityGmailDelete:
			if !policy.AllowWrite || !policy.AllowGmailDelete {
				continue
			}
		default:
			continue
		}
		if len(policy.AllowTools) > 0 && !mcpToolAllowed(tool, policy.AllowTools) {
			continue
		}
		if len(runtimeAllow) > 0 && !mcpToolAllowed(tool, runtimeAllow) {
			continue
		}
		tools = append(tools, tool)
	}
	return tools
}

func main() {
	var rows []map[string]any
	if err := json.NewDecoder(os.Stdin).Decode(&rows); err != nil {
		panic(err)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		r := map[string]any{"name": f["name"]}
		selected := make([]bool, 2)
		for i, side := range []string{"before", "after"} {
			get := func(k string) any { return f[side+"_"+k] }
			r[side+"_present"] = get("present")
			for _, k := range []string{"service", "risk", "capability"} {
				r[side+"_"+k] = get(k)
			}
			catalog = nil
			if get("present").(bool) {
				catalog = []mcpToolSpec{{f["name"].(string), get("service").(string), get("risk").(string), get("capability").(string)}}
			}
			list := func(k string) []string {
				v := get(k).([]any)
				a := make([]string, 0, len(v))
				for _, s := range v {
					a = append(a, s.(string))
				}
				return a
			}
			p := MCPPolicy{get("allow_write").(bool), get("allow_gmail_send").(bool), get("allow_gmail_delete").(bool), list("policy_selectors")}
			selected[i] = len(mcpFilterTools(p, list("runtime_selectors"), &RootFlags{get("readonly").(bool)})) == 1
			r[side+"_selected"] = selected[i]
		}
		before, after := selected[0], selected[1]
		r["added"] = !before && after
		r["removed"] = before && !after
		classification := "REMAINS_UNSELECTED"
		if before && after {
			classification = "REMAINS_SELECTED"
		}
		if !before && after {
			classification = "ADDED_TO_SELECTED_CATALOG"
		}
		if before && !after {
			classification = "REMOVED_FROM_SELECTED_CATALOG"
		}
		r["classification"] = classification
		out = append(out, r)
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
