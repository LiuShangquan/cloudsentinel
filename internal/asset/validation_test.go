package asset

import "testing"

func TestValidateHostAddress(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"10.0.0.1", true}, {"2001:db8::1", true}, {"service.internal", true},
		{"http://host", false}, {"host:8080", false}, {"host/path", false}, {"user@host", false}, {"-bad.example", false},
	}
	for _, test := range tests {
		if got := validateHostAddress(test.value) == nil; got != test.valid {
			t.Errorf("validateHostAddress(%q) valid=%v, want %v", test.value, got, test.valid)
		}
	}
}

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		kind, target string
		valid        bool
	}{
		{TypeHTTP, "https://example.internal/health", true},
		{TypeHTTP, "http://10.0.0.1:8080", true},
		{TypeHTTP, "http://user:pass@example.com", false},
		{TypeHTTP, "ftp://example.com", false},
		{TypeHTTP, "http://example.com:70000", false},
		{TypeTCP, "redis.internal:6379", true},
		{TypeTCP, "[2001:db8::1]:443", true},
		{TypeTCP, "redis.internal:70000", false},
		{TypeTCP, "missing-port", false},
	}
	for _, test := range tests {
		if got := validateTarget(test.kind, test.target) == nil; got != test.valid {
			t.Errorf("validateTarget(%q, %q) valid=%v, want %v", test.kind, test.target, got, test.valid)
		}
	}
}

func TestPagination(t *testing.T) {
	got := pagination(2, 20, 41)
	if got.TotalPages != 3 || got.Page != 2 || got.Total != 41 {
		t.Fatalf("pagination = %+v", got)
	}
}
