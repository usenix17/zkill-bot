package rules

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"

	"zkill-bot/internal/killmail"
)

// Mode controls how many rules are evaluated once a match is found.
type Mode string

const (
	ModeFirstMatch Mode = "first-match"
	ModeMultiMatch Mode = "multi-match"
)

// ActionConfig holds the action type and optional per-rule arguments.
type ActionConfig struct {
	Type string                 `yaml:"type"`
	Args map[string]interface{} `yaml:"args"`
}

// Rule is a single configured rule with its filter tree and actions.
type Rule struct {
	Name     string       `yaml:"name"`
	Enabled  bool         `yaml:"enabled"`
	Priority int          `yaml:"priority"`
	Filter   FilterNode   `yaml:"filter"`
	Actions  []ActionConfig `yaml:"actions"`
}

// RuleFile is the top-level YAML document.
type RuleFile struct {
	Mode  Mode   `yaml:"mode"`
	Rules []Rule `yaml:"rules"`
}

// RuleMatch is returned by Evaluate for each matched rule.
type RuleMatch struct {
	Rule    *Rule
	Actions []ActionConfig
}

// Load parses the YAML rules file at path and returns a validated RuleFile.
func Load(path string) (*RuleFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules: read %q: %w", path, err)
	}

	var rf RuleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("rules: parse %q: %w", path, err)
	}

	// Default mode
	if rf.Mode == "" {
		rf.Mode = ModeFirstMatch
	}
	if rf.Mode != ModeFirstMatch && rf.Mode != ModeMultiMatch {
		return nil, fmt.Errorf("rules: invalid mode %q (want %q or %q)", rf.Mode, ModeFirstMatch, ModeMultiMatch)
	}

	// Sort enabled rules by priority (lower number = higher priority).
	sort.SliceStable(rf.Rules, func(i, j int) bool {
		return rf.Rules[i].Priority < rf.Rules[j].Priority
	})

	return &rf, nil
}

// Evaluate tests km against all enabled rules in priority order.
// In first-match mode it returns as soon as one rule matches.
// In multi-match mode it returns all matching rules.
func Evaluate(km *killmail.Killmail, rf *RuleFile) []RuleMatch {
	var matches []RuleMatch
	for i := range rf.Rules {
		r := &rf.Rules[i]
		if !r.Enabled {
			continue
		}
		if matchFilter(km, &r.Filter) {
			matches = append(matches, RuleMatch{Rule: r, Actions: r.Actions})
			if rf.Mode == ModeFirstMatch {
				return matches
			}
		}
	}
	return matches
}
