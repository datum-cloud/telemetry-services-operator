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

// TestNATSSubjectsArePoPScoped guards the edge collectors' publish subjects
// against the hub's per-PoP leaf grant, which allows only
// o11y.<signal>.<cluster>.* -- a leaf publish outside it is dropped by the
// edge with no error on either side, so the whole path looks healthy while
// nothing crosses. See milo-os/telemetry#146 for the grant.
func TestNATSSubjectsArePoPScoped(t *testing.T) {
	collectors := map[string]bool{
		"collectors/node-agent-collector.yaml": true,
		"collectors/gateway-collector.yaml":    true,
	}
	for path := range collectors {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}

		var doc struct {
			Spec struct {
				Config struct {
					Processors map[string]struct {
						LogStatements []struct {
							Statements []string `yaml:"statements"`
						} `yaml:"log_statements"`
					} `yaml:"processors"`
					Exporters struct {
						NATS struct {
							Logs struct {
								Subject string `yaml:"subject"`
							} `yaml:"logs"`
						} `yaml:"nats"`
					} `yaml:"exporters"`
				} `yaml:"config"`
			} `yaml:"spec"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}

		// The attribute the nats exporter routes on must carry the cluster
		// token between the signal and the project.
		var subjectStmt string
		for _, proc := range doc.Spec.Config.Processors {
			for _, block := range proc.LogStatements {
				for _, stmt := range block.Statements {
					if strings.Contains(stmt, `"telemetry.subject"`) {
						subjectStmt = stmt
					}
				}
			}
		}
		if subjectStmt == "" {
			t.Errorf("%s: no statement sets telemetry.subject", path)
			continue
		}
		if !strings.Contains(subjectStmt, `resource.attributes["k8s.cluster.name"]`) {
			t.Errorf("%s: telemetry.subject omits the cluster token, so the hub's per-PoP grant silently drops it: %s",
				path, subjectStmt)
		}

		// The static fallback has to satisfy the same grant.
		if got := doc.Spec.Config.Exporters.NATS.Logs.Subject; !strings.Contains(got, "${cluster}") {
			t.Errorf("%s: nats exporter fallback subject %q omits the cluster token", path, got)
		}
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
