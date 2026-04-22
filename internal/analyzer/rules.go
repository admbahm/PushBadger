// Package analyzer loads risk rules and maps changed files to risk areas.
package analyzer

import (
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
	"pushbadger/config"
)

// rulesetFile is the top-level schema for risk_rules.yaml.
type rulesetFile struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

// Rule is a single entry in the ruleset.
type Rule struct {
	Area     string   `yaml:"area"`
	Priority int      `yaml:"priority"`
	Patterns []string `yaml:"patterns"`
}

// LoadRuleset reads and parses a ruleset file, returning the rules and the
// version field from the YAML.
// If path is empty the embedded default ruleset (config/risk_rules.yaml) is used.
// Rules are returned sorted by priority ascending, then area name ascending.
func LoadRuleset(path string) (rules []Rule, version int, err error) {
	var data []byte
	if path == "" {
		data = config.DefaultRules
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, 0, fmt.Errorf("opening rules file: %w", err)
		}
		defer f.Close()
		data, err = io.ReadAll(f)
		if err != nil {
			return nil, 0, fmt.Errorf("reading rules file: %w", err)
		}
	}
	return parseRuleset(data)
}

func parseRuleset(data []byte) ([]Rule, int, error) {
	var rs rulesetFile
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, 0, fmt.Errorf("parsing rules file: %w", err)
	}
	sort.Slice(rs.Rules, func(i, j int) bool {
		if rs.Rules[i].Priority != rs.Rules[j].Priority {
			return rs.Rules[i].Priority < rs.Rules[j].Priority
		}
		return rs.Rules[i].Area < rs.Rules[j].Area
	})
	return rs.Rules, rs.Version, nil
}
