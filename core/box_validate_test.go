package core

import (
	"strings"
	"testing"
)

// Box creation validates what upstream sing-box validates since 1.14.6: route
// rules and DNS references must point at something that exists, and the
// implicit fallbacks count as existing.
func TestBoxValidatesReferences(t *testing.T) {
	cases := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "route rule to a missing outbound",
			config:  `{"route":{"rules":[{"domain":["example.com"],"outbound":"missing"}]}}`,
			wantErr: "outbound not found: missing",
		},
		{
			name:   "route rule to the implicit direct fallback",
			config: `{"route":{"rules":[{"domain":["example.com"],"outbound":"direct"}]}}`,
		},
		{
			name:    "dns final to a missing server",
			config:  `{"dns":{"final":"missing"}}`,
			wantErr: "default DNS server not found: missing",
		},
		{
			name:   "dns rule to the implicit local fallback",
			config: `{"dns":{"rules":[{"domain":["example.com"],"server":"local"}]}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCore()
			t.Cleanup(func() { _ = c.Stop() })
			err := c.Start([]byte(tc.config))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Start rejected a valid config: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Start accepted an invalid config")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected %q, got %v", tc.wantErr, err)
			}
			if c.GetInstance() != nil {
				t.Error("a rejected config left an instance behind")
			}
		})
	}
}
