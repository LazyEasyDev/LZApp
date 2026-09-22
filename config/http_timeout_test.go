package config

import (
	"encoding/json"
	"testing"
)

func TestHTTPTimeoutDefaults(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		config AppConfig
	}{
		{name: "defaults", config: newDefaultConfig()},
		{name: "debug", config: debugAppConfig},
		{name: "release", config: releaseAppConfig},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			data, err := json.Marshal(testCase.config.HTTP)
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]json.RawMessage
			if err := json.Unmarshal(data, &values); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]int{
				"read_header_timeout_seconds": 10,
				"idle_timeout_seconds":        60,
				"read_timeout_seconds":        0,
				"write_timeout_seconds":       0,
				"shutdown_timeout_seconds":    60,
			} {
				var got int
				if err := json.Unmarshal(values[key], &got); err != nil {
					t.Errorf("invalid or missing %s: %v", key, err)
				} else if got != want {
					t.Errorf("%s = %d, want %d", key, got, want)
				}
			}
		})
	}
}
