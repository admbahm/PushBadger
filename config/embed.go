// Package config provides the embedded default risk rules.
package config

import _ "embed"

// DefaultRules is the embedded content of risk_rules.yaml.
// Use --rules to override with a custom file at runtime.
//
//go:embed risk_rules.yaml
var DefaultRules []byte
