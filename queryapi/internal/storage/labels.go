// SPDX-License-Identifier: AGPL-3.0-only

package storage

// LabelKind is where a LogQL label lives in the logs table.
type LabelKind int

const (
	LabelColumn LabelKind = iota
	LabelResourceAttribute
	LabelLogAttribute
)

// columns are the promoted labels backed by real columns. Everything else is
// an attribute lookup, so new attributes are queryable without a code change.
var columns = map[string]string{
	"service_name": "ServiceName",
	"severity":     "SeverityText",
	"trace_id":     "TraceId",
}

// Resolve maps a LogQL label to its storage location.
func Resolve(label string) (target string, kind LabelKind) {
	if col, ok := columns[label]; ok {
		return col, LabelColumn
	}
	return label, LabelLogAttribute
}
