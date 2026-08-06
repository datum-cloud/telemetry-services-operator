// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import "testing"

func TestConfigValidate(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}

	cases := map[string]Config{
		"zero default":      {DefaultLimit: 0, MaxLimit: 5000},
		"negative default":  {DefaultLimit: -1, MaxLimit: 5000},
		"zero max":          {DefaultLimit: 100, MaxLimit: 0},
		"default above max": {DefaultLimit: 10000, MaxLimit: 100},
	}
	for name, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}
