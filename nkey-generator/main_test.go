// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"strings"
	"testing"

	"github.com/nats-io/nkeys"
	"sigs.k8s.io/yaml"
)

func TestGenerateNKey(t *testing.T) {
	kp, err := generateNKey()
	if err != nil {
		t.Fatalf("generateNKey: %v", err)
	}

	// Seed must be a valid base32-encoded NKey user seed ("SU" prefix).
	if !strings.HasPrefix(string(kp.Seed), "SU") {
		t.Fatalf("seed does not start with SU prefix: %q", kp.Seed)
	}

	decoded, err := nkeys.FromSeed(kp.Seed)
	if err != nil {
		t.Fatalf("FromSeed: %v", err)
	}
	defer decoded.Wipe()

	pubFromSeed, err := decoded.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if pubFromSeed != kp.Public {
		t.Fatalf("public key mismatch: fromSeed=%q pair=%q", pubFromSeed, kp.Public)
	}
}

func TestNkeyUserAllowlist(t *testing.T) {
	u := nkeyUser("us-east-1", "UD5ULF5HDXSAB42CKHYDGDQ5E53AJNSXYW72MEYMHNVGAQKNRZS55H5N")

	if u["nkey"] != "UD5ULF5HDXSAB42CKHYDGDQ5E53AJNSXYW72MEYMHNVGAQKNRZS55H5N" {
		t.Fatalf("nkey not set")
	}
	if _, hasUser := u["user"]; hasUser {
		t.Fatalf("nkey user must not carry a user field")
	}

	publish := mustStrSlice(t, u, "publish")
	expected := []string{
		"o11y.logs.us-east-1.*",
		"o11y.metrics.us-east-1.*",
		"o11y.traces.us-east-1.*",
		"$JS.API.>",
		"$JS.hub.API.>",
	}
	if len(publish) != len(expected) {
		t.Fatalf("publish allow len=%d want %d: %v", len(publish), len(expected), publish)
	}
	for i, e := range expected {
		if publish[i] != e {
			t.Fatalf("publish[%d]=%q want %q", i, publish[i], e)
		}
	}

	// The whole point: never widen a leaf to publish any PoP's subjects.
	for _, allow := range publish {
		if allow == ">" || strings.HasPrefix(allow, "o11y.logs.*") {
			t.Fatalf("leaf allowlist widened beyond its own PoP: %q", allow)
		}
	}

	sub := mustStrSlice(t, u, "subscribe")
	if len(sub) != 1 || sub[0] != "_INBOX.>" {
		t.Fatalf("subscribe allow = %v, want [_INBOX.>]", sub)
	}
}

func TestStaticUsers(t *testing.T) {
	users := staticUsers()
	if len(users) != 2 {
		t.Fatalf("expected 2 static users, got %d", len(users))
	}

	// NACK has full access.
	nack := users[0]
	if nack["user"] != "CN=nack.nats.client" {
		t.Fatalf("nack user = %v", nack["user"])
	}
	if got := mustStrSlice(t, nack, "publish"); len(got) != 1 || got[0] != ">" {
		t.Fatalf("nack publish = %v, want [>]", got)
	}

	// Sink is restricted to JetStream + o11y subscribe.
	sink := users[1]
	if sink["user"] != "CN=o11y-sink-nats-client" {
		t.Fatalf("sink user = %v", sink["user"])
	}
}

func TestRenderedValuesRoundTrip(t *testing.T) {
	users := append(staticUsers(), nkeyUser("us-east-1", "UD5ULF5HDXSAB42CKHYDGDQ5E53AJNSXYW72MEYMHNVGAQKNRZS55H5N"))

	// Exercise the same marshal path writeConfigMap uses by rendering the
	// document to bytes and asserting key structural markers survive.
	doc := map[string]any{
		"config": map[string]any{
			"merge": map[string]any{
				"accounts": map[string]any{
					"O11Y": map[string]any{
						"jetstream": "enabled",
						"users":     users,
					},
				},
			},
		},
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		"O11Y:",
		"jetstream: enabled",
		"nkey: UD5ULF5",
		"o11y.logs.us-east-1.*",
		"CN=nack.nats.client",
		"CN=o11y-sink-nats-client",
	} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("rendered values missing %q:\n%s", want, out)
		}
	}
}

func mustStrSlice(t *testing.T, m map[string]any, dir string) []string {
	t.Helper()
	perms, ok := m["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("no permissions map")
	}
	d, ok := perms[dir].(map[string]any)
	if !ok {
		t.Fatalf("no %s map", dir)
	}
	allow, ok := d["allow"].([]any)
	if !ok {
		t.Fatalf("no allow slice")
	}
	out := make([]string, 0, len(allow))
	for _, v := range allow {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("allow item not a string")
		}
		out = append(out, s)
	}
	return out
}
