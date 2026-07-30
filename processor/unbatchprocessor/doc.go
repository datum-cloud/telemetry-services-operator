//go:generate make mdatagen

// Package unbatchprocessor splits every incoming batch of logs, metrics, or
// traces into one batch per individual record (log record, metric data
// point, or span), preserving each record's parent Resource and Scope.
package unbatchprocessor // import "go.datum.net/o11y/processor/unbatchprocessor"
