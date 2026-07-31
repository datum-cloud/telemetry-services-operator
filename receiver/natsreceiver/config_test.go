package natsreceiver

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid jetstream mode", func(*Config) {}, false},
		{"valid core nats mode", func(c *Config) { c.Stream = ""; c.ConsumerName = "" }, false},
		{"missing url", func(c *Config) { c.URL = "" }, true},
		{"stream without consumer_name", func(c *Config) { c.ConsumerName = "" }, true},
		{"consumer_name without stream", func(c *Config) { c.Stream = "" }, true},
		{"bad logs encoding", func(c *Config) { c.Logs.Encoding = "bogus" }, true},
		{"missing logs subject", func(c *Config) { c.Logs.Subject = "" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				URL:          "nats://localhost:4222",
				Stream:       "otlp",
				ConsumerName: "test-consumer",
				Logs:         SignalConfig{Subject: "otlp.logs", Encoding: "otlp_proto"},
			}
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
