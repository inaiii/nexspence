# Cleanup matching and image architecture verification

This fork fixes cleanup matching and multi-platform image builds. Baseline: upstream
v2.13.3 (`67619ad`); implementation based on main `9f0bc3c`. The relevant Dockerfile,
cleanup service, asset query and cleanup page were unchanged between those revisions.

## Published baseline evidence

The public `ghcr.io/nexspence/nexspence:v2.13.3` index resolves to:

```
sha256:f441f72bb0c1a6d8e425f1896a195c1360d907af76c31721c57f0b0b1b16cb01
linux/amd64: sha256:c1102daa0088f6075c938f38f35896332e441ac4c5d8977342aad187169604ea
linux/arm64: sha256:cabd9a97df0d018213ed0811116f1a1ff8105f3b70ae871ce565857f7d165041
```

Pulling by that index with `--platform linux/arm64`, creating a stopped container,
and copying `/app/nexspence` yields a 64-bit little-endian x86-64 ELF with
`e_machine=62`, despite the image configuration reporting `arm64`. AArch64 must
be 183. No execution or transparent translation is needed to establish this.

Both release.yml and main-image.yml request linux/amd64,linux/arm64. The builder
uses BUILDPLATFORM and cross-compiles with TARGETARCH, but `ARG TARGETARCH=amd64`
explicitly overrides BuildKit's automatic target argument. Removing the defaults
from TARGETOS and TARGETARCH lets BuildKit supply the actual target platform.

`Fork images` builds on native GitHub amd64 and arm64 runners, checks the image
config and ELF header, executes CLI help, starts the server against PostgreSQL,
and checks HTTP health. It uploads tested image archives, then publishes those
same archives (no rebuild) to Docker Hub and creates a multi-platform index.
Publication requires fork secrets `DOCKERHUB_USERNAME=inaiii` and
`DOCKERHUB_TOKEN`. The final anonymous manifest check requires a public repository.
Tags are `sha-<full commit>` and `cleanup-multiarch`; upstream release tags and
`latest` are not overwritten.

## What cleanup fields match

All cleanup path filters operate on the complete stored `assets.path` string.
For `registry.example.com/team/app:0.1`, the tag manifest path is
`/manifests/team/app/0.1`. It is neither the registry URL nor `app` nor `0.1` alone.
Maven paths look like `/com/example/lib/1.0/lib-1.0.jar`. Other formats use their
stored asset paths: inspect Preview for the exact representation.

| Field | Semantics |
| --- | --- |
| `criteria.pathPrefix` | Literal prefix, not glob/regex. |
| `criteria.nameGlob` | Existing SQL LIKE conversion: `*` any characters, `?` one character, including `/`. Matches the entire asset path; `%` and `_` are literal. |
| `criteria.nameRegex` | New optional Go regexp search on the full path, ANDed with glob and other criteria. |
| `scope.repositoryName` | If set, replaces the attached-repository list. Otherwise only attached repositories are scanned. |
| `scope.pathPrefix` | With a scoped repository, a nonempty value overrides `criteria.pathPrefix`. Empty falls back to criteria. It is not a second intersecting prefix. |

Existing glob policies keep their semantics. Regex is case sensitive by default,
uses Go's RE2-style syntax, and searches for a substring: use `^` and `$` for
boundaries. Groups, alternation, character classes and repetition are supported;
lookaround and backreferences are not. There is **no claim of full compatibility
with Nexus's Java regex engine**. Empty regex disables it. Non-string values,
invalid expressions and patterns over 4096 UTF-8 bytes produce HTTP 400 on create
and update, displayed in both wizard and edit forms. Persisted invalid regex also
fails closed during preview/execution. No schema migration is needed (criteria is JSONB).

Examples for numeric OCI tags across nested image names:

| Tag category | `nameRegex` |
| --- | --- |
| Major, e.g. 0 or 1 | `^/manifests/.+/[0-9]+$` |
| Minor, e.g. 0.1 or 12.34 | `^/manifests/.+/[0-9]+\.[0-9]+$` |
| Patch, e.g. 0.1.12 | `^/manifests/.+/[0-9]+\.[0-9]+\.[0-9]+$` |

These exclude latest, nightly, prerelease suffixes and extra version segments.
The final segment is isolated, so dots in image names do not affect version
matching. To select only major tags 0 and 1, use `^/manifests/.+/(0|1)$`.
For a specific image, replace `.+` with its escaped name, e.g. `team/app`.
In JSON, backslashes must be escaped (`\\.`); enter `\.` in the UI.

Regex runs in Go over streamed, SQL-filtered candidates rather than PostgreSQL
regex, ensuring validation and execution use the same engine. The 200-preview /
500-dry-run limit counts matches, not candidates. Real execution continues in
batches. A restrictive regex can scan many candidates; use a repository and
literal path prefix to narrow the scan. Existing age, retention and hosted OCI
reference protections continue to apply. Retention protects newest versions;
it does not override other filters to force deletion.

## Picker tracing and compatibility

Previously CleanupPage called `listPathTree(repo)`; the shared endpoint returned
Docker image namespace directories (`/team/app/`) for content-selector use. The
page submitted them unchanged as `scope.pathPrefix`. Both PreviewPolicy and
runPolicy overrode criteria.pathPrefix and passed this value into `a.path LIKE`,
which cannot match `/manifests/team/app/0.1`.

CleanupPage now requests `path-tree?purpose=cleanup`. For docker/oci the endpoint
uses manifest paths, authorizes in the original image namespace, then returns
`/manifests/team/app/`. Non-cleanup consumers keep image namespace paths. The
selected prefix is displayed and saved verbatim. Changing the scoped repository
clears the old path. **Existing saved `/team/app/` cleanup scopes are not silently
rewritten**: reselect the path using Browse, save, and Preview before running.
An image actually named `manifests/...` remains unambiguous in this representation.

Tests cover numeric version categories, malformed/unsupported patterns, API
validation, Docker and OCI picker results, UI submission/error display, prefix
override, glob+regex intersection, retention, unscoped preview, dry run and real
multi-batch deletion after more than 500 nonmatching assets. Run:

```sh
go test ./...
go test -tags integration ./internal/repository/postgres -count=1
cd frontend
npm ci
npm run build
npm run typecheck:test
npm test -- --run
```
