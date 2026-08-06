// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.datum.net/o11y/queryapi/internal/storage"
)

// parseTime accepts RFC 3339, a Unix timestamp in seconds or nanoseconds, or
// a relative duration meaning "that long before now". Empty returns fallback.
func parseTime(raw string, now, fallback time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if len(raw) > 12 { // nanoseconds
			return time.Unix(0, n).UTC(), nil
		}
		return time.Unix(n, 0).UTC(), nil
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return now.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", raw)
}

// parseLimit clamps to MaxLimit rather than rejecting, matching Loki.
func parseLimit(raw string, cfg Config) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return cfg.DefaultLimit
	}
	if n > cfg.MaxLimit {
		return cfg.MaxLimit
	}
	return n
}

func parseDirection(raw string) storage.Direction {
	if strings.TrimSpace(raw) == string(storage.DirectionForward) {
		return storage.DirectionForward
	}
	return storage.DirectionBackward
}
