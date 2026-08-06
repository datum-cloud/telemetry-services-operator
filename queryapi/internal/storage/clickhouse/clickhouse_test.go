// SPDX-License-Identifier: AGPL-3.0-only

package clickhouse

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"go.datum.net/o11y/queryapi/internal/miloauth"
	"go.datum.net/o11y/queryapi/internal/storage"
)

// stubConn satisfies driver.Conn by embedding it. Only Close is implemented:
// the methods under test short-circuit before touching the connection, so any
// other call is a bug this test should surface as a nil-interface panic.
type stubConn struct {
	driver.Conn
}

func (stubConn) Close() error { return nil }

// TestQueriesAreNotImplemented pins the contract for this round: the backend
// connects and reports health, but does not query.
func TestQueriesAreNotImplemented(t *testing.T) {
	store := newStoreWithConn(stubConn{})

	ctx := miloauth.WithProject(context.Background(), "proj-abc")
	tr := storage.TimeRange{Start: time.Unix(0, 0), End: time.Unix(1, 0)}

	if _, err := store.QueryLogs(ctx, storage.LogQuery{Range: tr, Limit: 1}); !errors.Is(err, storage.ErrNotImplemented) {
		t.Errorf("QueryLogs = %v, want ErrNotImplemented", err)
	}
	if _, err := store.LabelNames(ctx, tr); !errors.Is(err, storage.ErrNotImplemented) {
		t.Errorf("LabelNames = %v, want ErrNotImplemented", err)
	}
	if _, err := store.LabelValues(ctx, "severity", tr); !errors.Is(err, storage.ErrNotImplemented) {
		t.Errorf("LabelValues = %v, want ErrNotImplemented", err)
	}
	if _, err := store.Series(ctx, nil, tr); !errors.Is(err, storage.ErrNotImplemented) {
		t.Errorf("Series = %v, want ErrNotImplemented", err)
	}
}

// TestNoProjectBeatsNotImplemented proves tenancy is checked before anything
// else, so the backend cannot be coaxed into an unscoped query.
func TestNoProjectBeatsNotImplemented(t *testing.T) {
	store := newStoreWithConn(stubConn{})

	if _, err := store.QueryLogs(context.Background(), storage.LogQuery{Limit: 1}); !errors.Is(err, storage.ErrNoProject) {
		t.Errorf("QueryLogs without a project = %v, want ErrNoProject", err)
	}
	if _, err := store.LabelNames(context.Background(), storage.TimeRange{}); !errors.Is(err, storage.ErrNoProject) {
		t.Errorf("LabelNames without a project = %v, want ErrNoProject", err)
	}
}

// TestPingFailsWhenUnreachable exercises the real constructor, including TLS
// assembly, against a host that cannot resolve.
func TestPingFailsWhenUnreachable(t *testing.T) {
	certFile, keyFile, caFile := writeTestCerts(t)

	store, err := New(Config{
		Host: "clickhouse.invalid", Port: 9440, User: "queryapi", Database: "o11y",
		TLSCertFile: certFile, TLSKeyFile: keyFile, TLSCAFile: caFile,
	})
	if err != nil {
		t.Fatalf("New with generated certificates: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Logf("close store: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err == nil {
		t.Fatal("Ping to an unresolvable host succeeded, want error")
	}
}

// writeTestCerts writes a self-signed client certificate, its key, and a CA
// file (the same certificate) into a temp dir.
func writeTestCerts(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "queryapi"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(crand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "tls.crt")
	keyFile = filepath.Join(dir, "tls.key")
	caFile = filepath.Join(dir, "ca.crt")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	for path, contents := range map[string][]byte{
		certFile: certPEM,
		keyFile:  keyPEM,
		caFile:   certPEM,
	} {
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return certFile, keyFile, caFile
}
