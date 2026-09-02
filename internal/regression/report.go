package regression

import (
	"fmt"
	"strings"
)

// RenderText renders a human-readable regression report. It is deliberately
// terse: it names the failing metrics and their baseline/current/delta/threshold
// values instead of dumping raw benchmark data.
func (r *Report) RenderText() string {
	var b strings.Builder
	b.WriteString("Regression Report\n\n")
	fmt.Fprintf(&b, "Overall: %s\n", r.OverallStatus)
	fmt.Fprintf(&b, "Baseline: %s\n", r.BaselineID)
	fmt.Fprintf(&b, "Current:  %s\n", r.CurrentID)

	if r.OverallStatus == StatusPass {
		b.WriteString("\nAll monitored quality metrics are within regression thresholds.\n")
		return b.String()
	}

	b.WriteString("\nFailed Metrics:\n")
	for _, mc := range r.Comparisons {
		if mc.Status != StatusFail {
			continue
		}
		fmt.Fprintf(&b, "\n%s\n", mc.DisplayName)
		switch {
		case mc.Missing:
			b.WriteString("baseline or current metric is missing\n")
		case mc.NonFinite:
			b.WriteString("baseline or current metric is not a finite number\n")
		default:
			fmt.Fprintf(&b, "baseline = %.4f\n", *mc.Baseline)
			fmt.Fprintf(&b, "current  = %.4f\n", *mc.Current)
			fmt.Fprintf(&b, "delta    = %+.4f\n", *mc.Delta)
			fmt.Fprintf(&b, "threshold = %+.4f\n", -mc.AllowedDrop)
		}
	}
	b.WriteString("\n")
	b.WriteString(r.Summary)
	b.WriteString("\n")
	return b.String()
}
