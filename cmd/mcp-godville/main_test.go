package main

import "testing"

func TestIsWildcardOrPublicHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"", true},
		{"0.0.0.0", true},
		{"::", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"localhost", false},
		{"192.0.2.1", true},       // RFC 5737 documentation IP, non-loopback
		{"203.0.113.5", true},     // RFC 5737, non-loopback
		{"2001:db8::1", true},     // RFC 3849 documentation, non-loopback
		{"example.invalid", true}, // DNS name → warn (resolution could be anything)
	}

	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := isWildcardOrPublicHost(tc.host)
			if got != tc.want {
				t.Errorf("isWildcardOrPublicHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
