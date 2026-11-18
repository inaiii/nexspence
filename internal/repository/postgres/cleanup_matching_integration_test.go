//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexspence-oss/nexspence/internal/domain"
	"github.com/nexspence-oss/nexspence/internal/service"
	"github.com/nexspence-oss/nexspence/internal/testutil"
	"github.com/nexspence-oss/nexspence/internal/testutil/pgtest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCleanupRegexPreviewAndExecution(t *testing.T) {
	pool := pgtest.Pool(t)
	pgtest.Truncate(t, pool, "blob_stores", "repositories", "components")
	ctx := context.Background()
	p := makeAssetParent(t, ctx, "regex")
	_, err := pool.Exec(ctx, `UPDATE repositories SET format='oci' WHERE id=$1`, p.RepositoryID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE components SET format='oci' WHERE id=$1`, p.ComponentID)
	require.NoError(t, err)
	assets := NewAssetRepo(pool)
	// Nonmatches must not consume either the preview or execution batch limit.
	for i := 0; i < 510; i++ {
		require.NoError(t, assets.Create(ctx, makeAsset(p, fmt.Sprintf("/manifests/team/a.b/nightly-%d", i))))
	}
	for i := 0; i < 505; i++ {
		require.NoError(t, assets.Create(ctx, makeAsset(p, fmt.Sprintf("/manifests/team/a.b/1.%d", i))))
	}
	require.NoError(t, assets.Create(ctx, makeAsset(p, "/manifests/other/app/1.2")))
	policy := &domain.CleanupPolicy{ID: "11111111-1111-4111-8111-111111111111", Name: "minor", Format: "oci", Enabled: true, DryRun: true,
		Criteria: map[string]any{"pathPrefix": "/does-not-match/", "nameGlob": "/manifests/*", "nameRegex": `^/manifests/.+/[0-9]+\.[0-9]+$`},
		Scope:    domain.CleanupScope{RepositoryName: p.RepoName, PathPrefix: "/manifests/team/a.b/"},
	}
	policies := testutil.NewCleanupPolicyRepo(policy)
	svc := service.NewCleanupService(policies, NewRepositoryRepo(pool), assets, NewBlobStoreRepo(pool), testutil.NewBlobStore(), zap.NewNop().Sugar())
	preview, err := svc.PreviewPolicy(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Equal(t, 200, preview.TotalCount)
	for _, a := range preview.Assets {
		require.Contains(t, a.Path, "/manifests/team/a.b/1.")
	}
	dry, err := svc.RunPolicyResult(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Equal(t, 500, dry.Deleted)
	policy.DryRun = false
	require.NoError(t, policies.Update(ctx, policy))
	run, err := svc.RunPolicyResult(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Equal(t, 505, run.Deleted)
	remaining, err := assets.ListByRepoAndPath(ctx, p.RepoName, "")
	require.NoError(t, err)
	require.Len(t, remaining, 511)
	preview, err = svc.PreviewPolicy(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Zero(t, preview.TotalCount)
	// Malformed persisted patterns fail closed in both paths.
	policy.Criteria["nameRegex"] = "["
	require.NoError(t, policies.Update(ctx, policy))
	_, err = svc.PreviewPolicy(ctx, "11111111-1111-4111-8111-111111111111")
	require.ErrorContains(t, err, "nameRegex")
	_, err = svc.RunPolicyResult(ctx, "11111111-1111-4111-8111-111111111111")
	require.ErrorContains(t, err, "nameRegex")
	// Unattached preview must never scan other repositories.
	policy.Criteria = map[string]any{}
	policy.Scope = domain.CleanupScope{}
	require.NoError(t, policies.Update(ctx, policy))
	preview, err = svc.PreviewPolicy(ctx, "11111111-1111-4111-8111-111111111111")
	require.NoError(t, err)
	require.Zero(t, preview.TotalCount)
}

func TestCleanupRegexRetentionAndIntersection(t *testing.T) {
	pool := pgtest.Pool(t)
	pgtest.Truncate(t, pool, "blob_stores", "repositories", "components")
	ctx := context.Background()
	name := makeOCIImageRepo(t, ctx, "regex_retain", domain.TypeHosted)
	assets := NewAssetRepo(pool)
	got, err := assets.ListStale(ctx, "docker", []string{name}, 0, 0, "/manifests/bb/", "*v?", `/v[0-9]+$`, 1, 1)
	require.NoError(t, err)
	require.Equal(t, []string{"/manifests/bb/v1"}, stalePaths(got))
	got, err = assets.ListStale(ctx, "docker", []string{name}, 0, 0, "/manifests/bb/", "*v2", `/v1$`, 0, 10)
	require.NoError(t, err)
	require.Empty(t, got)
	got, err = assets.ListStale(ctx, "docker", nil, 0, 0, "", "", "", 0, 10)
	require.NoError(t, err)
	require.Empty(t, got)
}
