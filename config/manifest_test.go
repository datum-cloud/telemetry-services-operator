// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestServerAuthCertsHaveDNSNames guards against a cert-manager CSI volume
// requesting server auth with no SAN names: cert-manager issues the
// certificate anyway, but Go's TLS client verifier requires a SAN match and
// ignores the deprecated CN field entirely, so every such client rejects the
// handshake. See milo-os/telemetry#148.
func TestServerAuthCertsHaveDNSNames(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		for {
			var doc map[string]any
			if decErr := dec.Decode(&doc); decErr != nil {
				break
			}
			checkCSIVolumes(t, path, doc)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk config/: %v", err)
	}
}

func checkCSIVolumes(t *testing.T, path string, node any) {
	switch v := node.(type) {
	case map[string]any:
		if driver, _ := v["driver"].(string); driver == "csi.cert-manager.io" {
			attrs, _ := v["volumeAttributes"].(map[string]any)
			usages, _ := attrs["csi.cert-manager.io/key-usages"].(string)
			if strings.Contains(usages, "server auth") {
				dnsNames, _ := attrs["csi.cert-manager.io/dns-names"].(string)
				if strings.TrimSpace(dnsNames) == "" {
					t.Errorf("%s: CSI cert-manager volume (common-name %v) requests server auth with no dns-names -- cert-manager issues it with no SAN, which every Go TLS client rejects",
						path, attrs["csi.cert-manager.io/common-name"])
				}
			}
		}
		for _, child := range v {
			checkCSIVolumes(t, path, child)
		}
	case []any:
		for _, child := range v {
			checkCSIVolumes(t, path, child)
		}
	}
}
