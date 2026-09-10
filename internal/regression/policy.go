package regression

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// DefaultAllowedDrop is the built-in regression tolerance. It is a positive
// magnitude: a higher-is-better metric passes when
//
//	current >= baseline - AllowedDrop
//
// No official threshold is specified by the repository, so 0.02 is the
// documented default and every metric may be overridden in a policy file.
const DefaultAllowedDrop = 0.02

// Policy is the centralized regression threshold configuration. It is explicit
// rather than scattered magic numbers so a reviewer can see exactly how much
// regression is tolerated per metric.
type Policy struct {
	// DefaultAllowedDrop applies to every metric without a specific override.
	DefaultAllowedDrop float64
	// AllowedDrop overrides DefaultAllowedDrop for individual metrics.
	AllowedDrop map[MetricKey]float64
}

// DefaultPolicy returns the built-in policy: every metric tolerates an absolute
// drop of DefaultAllowedDrop. Threshold semantics are uniform across all twelve
// metrics (absolute drop, not percentage).
func DefaultPolicy() Policy {
	return Policy{DefaultAllowedDrop: DefaultAllowedDrop, AllowedDrop: map[MetricKey]float64{}}
}

// AllowedDropFor returns the allowed drop for a single metric.
func (p Policy) AllowedDropFor(key MetricKey) float64 {
	if p.AllowedDrop != nil {
		if v, ok := p.AllowedDrop[key]; ok {
			return v
		}
	}
	return p.DefaultAllowedDrop
}

// Validate reports whether the policy is usable. Thresholds must be finite and
// non-negative; a negative threshold would silently invert the comparison, so it
// is rejected rather than ignored.
func (p Policy) Validate() error {
	if err := validateDrop("default_allowed_drop", p.DefaultAllowedDrop); err != nil {
		return err
	}
	for key, v := range p.AllowedDrop {
		if !knownMetric(key) {
			return fmt.Errorf("unknown metric %q in policy", key)
		}
		if err := validateDrop(fmt.Sprintf("allowed_drop[%q]", key), v); err != nil {
			return err
		}
	}
	return nil
}

func validateDrop(field string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return fmt.Errorf("%s must be a finite non-negative number, got %v", field, v)
	}
	return nil
}

// policyFile is the JSON shape of a policy file. Keys are metric identifiers.
type policyFile struct {
	DefaultAllowedDrop float64            `json:"default_allowed_drop"`
	AllowedDrop        map[string]float64 `json:"allowed_drop"`
}

// LoadPolicy parses a policy JSON file and validates it.
func LoadPolicy(r io.Reader) (Policy, error) {
	dec := json.NewDecoder(r)
	var file policyFile
	if err := dec.Decode(&file); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	allowed := make(map[MetricKey]float64, len(file.AllowedDrop))
	for key, v := range file.AllowedDrop {
		allowed[MetricKey(key)] = v
	}
	policy := Policy{DefaultAllowedDrop: file.DefaultAllowedDrop, AllowedDrop: allowed}
	if err := policy.Validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}
