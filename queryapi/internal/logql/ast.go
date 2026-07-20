// SPDX-License-Identifier: AGPL-3.0-only

// Package logql implements the LogQL subset accepted by the query layer:
// label matchers and line filters. Metric-style aggregations, parser
// expressions, and line/label formatting are explicitly out of scope — see
// datum-cloud/enhancements, enhancements/telemetry/query-layer/README.md,
// "HTTP API contract" section, for what is supported and why.
package logql

// MatchOperator is a LogQL label-matcher operator.
type MatchOperator string

const (
	MatchEqual    MatchOperator = "="
	MatchNotEqual MatchOperator = "!="
)

// LineFilterOperator is a LogQL line-filter operator.
type LineFilterOperator string

const (
	LineContains       LineFilterOperator = "|="
	LineNotContains    LineFilterOperator = "!="
	LineMatchesRegexp  LineFilterOperator = "|~"
	LineNotMatchRegexp LineFilterOperator = "!~"
)

// LabelMatcher constrains a query to log lines whose resolved label value
// compares against Value using Operator. Label maps to a ClickHouse column
// (service_name, severity) or a JSON path lookup against
// LogAttributes/ResourceAttributes — see the SQL translation rules in the
// query-layer design doc.
type LabelMatcher struct {
	Label    string
	Operator MatchOperator
	Value    string
}

// LineFilter constrains a query to log lines whose Body matches Value under
// Operator.
type LineFilter struct {
	Operator LineFilterOperator
	Value    string
}

// Query is the parsed form of a LogQL query string.
type Query struct {
	Matchers []LabelMatcher
	Filters  []LineFilter
}
