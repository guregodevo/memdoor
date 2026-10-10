package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// memdoor guards: the rules that keep an agent out of what it must not touch.
// A rule is a tool name (or * for any) and a regexp over the call's input;
// a matching call is refused with the rule's message, which the agent reads
// and works around. The rules were a workspace setting set with a curl and a
// token (gateway/tool_guards.go), which nobody found, the author included
// (Greg, 2026-10-11: "no one know even myself"). Same setting, a command.
var guardsCmd = &cobra.Command{
	Use:   "guards",
	Short: "Rules that block a tool call: never .env, never push, never sudo",
	Long: `The rules that keep the agent out of what it must not touch.

A rule names a tool ("bash", "apply_patch", "*" for any) and a regular
expression matched against the call's input. A matching call is refused
with the rule's message, and the agent reads it and works another way.

  memdoor guards                                            the rules
  memdoor guards add --tool '*' --pattern '\.env\b' --message 'secrets stay out of reach'
  memdoor guards add --tool bash --pattern '\bgit push\b' --message 'pushing is mine'
  memdoor guards add --tool bash --pattern '\bsudo\b'
  memdoor guards remove 2
  memdoor guards clear

The rules apply to every agent in the workspace, on every turn, however
it was started (a window, a schedule, an editor). Setting them takes an
admin's sign-in; the first account on a machine is one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rules, err := readGuards(NewClient())
		if err != nil {
			return err
		}
		if len(rules) == 0 {
			fmt.Println("No guards. Add one:")
			fmt.Println(`  memdoor guards add --tool bash --pattern '\bgit push\b' --message 'pushing is mine'`)
			return nil
		}
		for i, r := range rules {
			fmt.Println(formatGuard(i+1, r))
		}
		return nil
	},
}

var guardsAddCmd = &cobra.Command{
	Use:   "add --pattern <regexp> [--tool <name>] [--message <text>]",
	Short: "Add a rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		tool, _ := cmd.Flags().GetString("tool")
		pattern, _ := cmd.Flags().GetString("pattern")
		message, _ := cmd.Flags().GetString("message")
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("--pattern is the rule: a Go regular expression over the call's input, like '\\bgit push\\b'")
		}
		c := NewClient()
		rules, err := readGuards(c)
		if err != nil {
			return err
		}
		rules = append(rules, guardRule{Tool: tool, Pattern: pattern, Message: message})
		if err := writeGuards(c, rules); err != nil {
			return err
		}
		fmt.Println(formatGuard(len(rules), rules[len(rules)-1]))
		return nil
	},
}

var guardsRemoveCmd = &cobra.Command{
	Use:   "remove <n>",
	Short: "Remove rule n (the number `memdoor guards` shows)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := NewClient()
		rules, err := readGuards(c)
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 || n > len(rules) {
			return fmt.Errorf("no rule %s: there are %d (memdoor guards lists them)", args[0], len(rules))
		}
		gone := rules[n-1]
		rules = append(rules[:n-1], rules[n:]...)
		if err := writeGuards(c, rules); err != nil {
			return err
		}
		fmt.Printf("removed: %s\n", formatGuard(n, gone))
		return nil
	},
}

var guardsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove every rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := writeGuards(NewClient(), nil); err != nil {
			return err
		}
		fmt.Println("No guards.")
		return nil
	},
}

// guardRule mirrors gateway/tool_guards.go's stored shape.
type guardRule struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern"`
	Message string `json:"message,omitempty"`
}

const guardsSettingKey = "tool_guards"

// guardRulesFrom decodes the setting's value; "" is no rules.
func guardRulesFrom(raw string) ([]guardRule, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var rules []guardRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("the %s setting is not a JSON list of rules: %w", guardsSettingKey, err)
	}
	return rules, nil
}

// guardsSetting encodes rules for the setting, refusing a rule the gateway
// would silently drop: an empty pattern or one that is not a regexp.
func guardsSetting(rules []guardRule) (string, error) {
	if len(rules) == 0 {
		return "", nil
	}
	for i, r := range rules {
		if strings.TrimSpace(r.Pattern) == "" {
			return "", fmt.Errorf("rule %d has no pattern", i+1)
		}
		if _, err := regexp.Compile(r.Pattern); err != nil {
			return "", fmt.Errorf("rule %d: %q is not a Go regular expression: %v", i+1, r.Pattern, err)
		}
		if rules[i].Tool == "" {
			rules[i].Tool = "*"
		}
	}
	b, err := json.Marshal(rules)
	return string(b), err
}

func formatGuard(n int, r guardRule) string {
	tool := r.Tool
	if tool == "" {
		tool = "*"
	}
	s := fmt.Sprintf("%d. %-12s %s", n, tool, r.Pattern)
	if r.Message != "" {
		s += "  — " + r.Message
	}
	return s
}

func readGuards(c *Client) ([]guardRule, error) {
	resp, err := c.Do(http.MethodGet, "/api/workspace/settings", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the workspace settings: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("reading the workspace settings: %w", err)
	}
	return guardRulesFrom(out.Settings[guardsSettingKey])
}

func writeGuards(c *Client, rules []guardRule) error {
	value, err := guardsSetting(rules)
	if err != nil {
		return err
	}
	resp, err := c.Do(http.MethodPut, "/api/workspace/settings", map[string]string{guardsSettingKey: value})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("setting guards takes an admin's sign-in (memdoor auth login-direct); the gateway said %s", resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("writing the guards: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func init() {
	guardsAddCmd.Flags().String("tool", "*", "the tool the rule watches: bash, apply_patch, write_file, … or * for any")
	guardsAddCmd.Flags().String("pattern", "", "a Go regular expression over the call's input")
	guardsAddCmd.Flags().String("message", "", "what the agent reads when the rule refuses a call")
	guardsCmd.AddCommand(guardsAddCmd, guardsRemoveCmd, guardsClearCmd)
	rootCmd.AddCommand(guardsCmd)
}
