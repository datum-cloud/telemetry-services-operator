// SPDX-License-Identifier: AGPL-3.0-only

package logql

import "errors"

// ErrNotImplemented is returned by Parse until the label-matcher/line-filter
// grammar is implemented. Handlers already call Parse so the SQL translation
// work can slot in without changing call sites.
var ErrNotImplemented = errors.New("logql: parser not yet implemented")

// Parse parses a LogQL query string into the supported subset (label
// matchers and line filters). Constructs outside that subset — metric-style
// aggregations, parser expressions, line_format/label_format — must be
// rejected here once implemented, not silently ignored.
func Parse(raw string) (*Query, error) {
	return nil, ErrNotImplemented
}
