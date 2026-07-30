// Hand-written placeholder for what `mdatagen` would generate from
// metadata.yaml. Swap for real codegen once mdatagen tooling is wired up.
package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("unbatch")
	ScopeName = "go.datum.net/o11y/processor/unbatchprocessor"
)

const (
	LogsStability    = component.StabilityLevelDevelopment
	MetricsStability = component.StabilityLevelDevelopment
	TracesStability  = component.StabilityLevelDevelopment
)
