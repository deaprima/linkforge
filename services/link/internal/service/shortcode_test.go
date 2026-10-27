package service

import (
	"strings"
	"testing"
)

func TestGenerateShortCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := generateShortCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != shortCodeLength {
			t.Fatalf("len(%q) = %d, want %d", code, len(code), shortCodeLength)
		}
		for _, r := range code {
			if !strings.ContainsRune(charset, r) {
				t.Fatalf("code %q contains char %q outside charset", code, r)
			}
		}
		seen[code] = true
	}
	// 62^6 ≈ 5.6e10 kombinasi; 1000 sampel seharusnya praktis tidak pernah bentrok
	if len(seen) < 999 {
		t.Errorf("too many duplicates: %d unique of 1000", len(seen))
	}
}

func TestValidateCustomAlias(t *testing.T) {
	tests := []struct {
		alias   string
		wantErr bool
	}{
		{"abc", false},
		{"my-alias_2026", false},
		{strings.Repeat("a", 64), false},
		{"ab", true},
		{strings.Repeat("a", 65), true},
		{"has space", true},
		{"slash/alias", true},
		{"emoji😀", true},
		{"api", true},
		{"Admin", true},
		{"healthz", true},
		{"readyz", true},
		{"metrics", true},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			err := validateCustomAlias(tt.alias)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCustomAlias(%q) err = %v, wantErr %v", tt.alias, err, tt.wantErr)
			}
		})
	}
}
