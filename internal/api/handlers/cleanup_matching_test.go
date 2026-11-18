package handlers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupRegexValidation(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, value := range []any{`[`, `(?=x)`, `(a)\1`, 42, nil, strings.Repeat("x", 4097)} {
			r, _, _, _ := mountCleanup(t)
			url := "/service/rest/v1/cleanup-policies"
			if method == http.MethodPut {
				url += "/p"
			}
			rec := do(t, r, method, url, map[string]any{"name": "p", "criteria": map[string]any{"nameRegex": value}})
			assert.Equal(t, 400, rec.Code)
			assert.Contains(t, rec.Body.String(), "criteria.nameRegex")
		}
	}
	r, _, _, _ := mountCleanup(t)
	rec := do(t, r, http.MethodPost, "/service/rest/v1/cleanup-policies", map[string]any{"name": "p", "criteria": map[string]any{"nameGlob": "/manifests/*", "nameRegex": `/[0-9]+$`}})
	require.Equal(t, 201, rec.Code)
	var got domain.CleanupPolicy
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, `/[0-9]+$`, got.Criteria["nameRegex"])
}

func TestCleanupPathTreeOCIUsesAssetPrefixes(t *testing.T) {
	for _, format := range []domain.RepoFormat{domain.FormatDocker, domain.FormatOCI} {
		r, repos, _, assets, _, _ := mountBrowse(t)
		seedRepo(t, repos, &domain.Repository{ID: "d1", Name: "images", Format: format, Type: domain.TypeHosted})
		require.NoError(t, assets.Create(testContext(), &domain.Asset{Repository: "images", Path: "/manifests/team/a.b/0.1"}))
		rec := do(t, r, http.MethodGet, "/api/v1/browse/repositories/images/path-tree?purpose=cleanup", nil)
		require.Equal(t, 200, rec.Code)
		var got struct {
			Paths []string `json:"paths"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Contains(t, got.Paths, "/manifests/team/a.b/")
		assert.NotContains(t, got.Paths, "/team/a.b/")
	}
}
