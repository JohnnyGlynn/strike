package config

import "testing"

func TestValidatePorts(t *testing.T) {
	cases := map[string]struct {
		client, federation         int
		wantClient, wantFederation int
		error                      bool
	}{
		"defaults-when-unset": {
			client: 0, federation: 0,
			wantClient: DefaultClientPort, wantFederation: DefaultFederationPort,
		},
		"explicit-ports-kept": {
			client: 18080, federation: 19090,
			wantClient: 18080, wantFederation: 19090,
		},
		"one-set-one-defaulted": {
			client: 18080, federation: 0,
			wantClient: 18080, wantFederation: DefaultFederationPort,
		},
		"privileged-rejected": {
			client: 443, federation: 19090,
			error: true,
		},
		"above-range-rejected": {
			client: 18080, federation: 65536,
			error: true,
		},
		"collision-rejected": {
			client: 18080, federation: 18080,
			error: true,
		},
		"collision-with-default-rejected": {
			client: DefaultFederationPort, federation: 0,
			error: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := ServerConfig{ClientPort: tc.client, FederationPort: tc.federation}

			err := c.validatePorts("client_port", "federation_port")
			if tc.error {
				if err == nil {
					t.Fatalf("expected an error, got none (ports %d/%d)", c.ClientPort, c.FederationPort)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if c.ClientPort != tc.wantClient {
				t.Errorf("client port: got %d, want %d", c.ClientPort, tc.wantClient)
			}

			if c.FederationPort != tc.wantFederation {
				t.Errorf("federation port: got %d, want %d", c.FederationPort, tc.wantFederation)
			}
		})
	}
}

func TestPortFromEnv(t *testing.T) {
	cases := map[string]struct {
		set   bool
		value string
		want  int
		error bool
	}{
		"unset-is-zero":    {set: false, want: 0},
		"empty-is-zero":    {set: true, value: "", want: 0},
		"parses-number":    {set: true, value: "18080", want: 18080},
		"rejects-garbage":  {set: true, value: "eighty-eighty", error: true},
		"rejects-trailing": {set: true, value: "8080/tcp", error: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.set {
				t.Setenv("STRIKE_TEST_PORT", tc.value)
			}

			got, err := portFromEnv("STRIKE_TEST_PORT")
			if tc.error {
				if err == nil {
					t.Fatalf("expected an error for %q, got %d", tc.value, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
