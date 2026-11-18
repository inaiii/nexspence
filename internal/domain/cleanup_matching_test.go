package domain

import (
	"strings"
	"testing"
)

func TestCleanupNameRegexp(t *testing.T) {
	for _, tc := range []struct {
		pattern         string
		matches, misses []string
	}{
		{`^/manifests/.+/[0-9]+$`, []string{"/manifests/team/app/0", "/manifests/a.b/nested/app/1"}, []string{"/manifests/app/0.1", "/manifests/app/latest"}},
		{`^/manifests/.+/[0-9]+\.[0-9]+$`, []string{"/manifests/app/0.1", "/manifests/a.b/team/app/12.34"}, []string{"/manifests/app/0.1.12", "/manifests/app/nightly", "/manifests/app/1.2-rc1"}},
		{`^/manifests/(team/)?app/[0-9]+\.[0-9]+\.[0-9]+$`, []string{"/manifests/team/app/0.1.12"}, []string{"/manifests/team/app/0.1.12-rc1", "/manifests/team/app/0.1.12/extra"}},
	} {
		re, err := CleanupNameRegexp(map[string]any{"nameRegex": tc.pattern})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range tc.matches {
			if !re.MatchString(p) {
				t.Errorf("%s should match %s", tc.pattern, p)
			}
		}
		for _, p := range tc.misses {
			if re.MatchString(p) {
				t.Errorf("%s should not match %s", tc.pattern, p)
			}
		}
	}
	for _, value := range []any{42, nil, `[`, `(?=x)`, `(x)\1`, strings.Repeat("x", 4097)} {
		if _, err := CleanupNameRegexp(map[string]any{"nameRegex": value}); err == nil {
			t.Errorf("accepted invalid regex %#v", value)
		}
	}
	for _, c := range []map[string]any{nil, {}, {"nameRegex": ""}} {
		re, err := CleanupNameRegexp(c)
		if err != nil || re != nil {
			t.Fatalf("empty: %v %v", re, err)
		}
	}
}
