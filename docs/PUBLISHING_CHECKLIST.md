# Publishing Checklist

SecretSync releases are automated from `main` with release-please and
GoReleaser. Do not hand-edit versions, changelog entries, release tags, or
GitHub releases during the normal release path.

## Release Model

- `release-please` owns version detection, changelog updates, release PRs, and
  Git tags.
- The root package name is `secrets-sync`, so component release tags use the
  `secrets-sync-vX.Y.Z` shape.
- GoReleaser runs only after release-please reports that a release was created.
- GoReleaser builds binary archives and checksums. Container and Marketplace
  publication are separate release surfaces.
- The Docker action currently references `docker://jbcom/secretssync:v1` from
  `action.yml`; digest pinning should be added only when release automation can
  refresh that digest reliably.

## Maintainer Preflight

Run these before merging a release PR or manually dispatching release workflow
diagnostics:

```bash
go test ./...
go build -o bin/secretsync ./cmd/secretsync
goreleaser check
docker build -t secretsync-test .
```

If `goreleaser` is not installed locally, use the pinned workflow action as the
source of truth and verify the GitHub check output.

## Workflow Hygiene

- Keep `.github/workflows/*.yml` actions pinned to exact commit SHAs.
- Update the adjacent version comments when refreshing an action SHA.
- Use `gh` to verify latest stable action releases before changing pins.
- Do not grant broad workflow permissions; keep top-level `permissions: {}` and
  add job-scoped permissions only where needed.

Current workflow action pins:

| Action | Stable version | Commit SHA |
| --- | --- | --- |
| `actions/checkout` | `v6.0.3` | `df4cb1c069e1874edd31b4311f1884172cec0e10` |
| `actions/setup-go` | `v6.4.0` | `4a3601121dd01d1626a1e23e37211e3254c1c06c` |
| `googleapis/release-please-action` | `v5.0.0` | `45996ed1f6d02564a971a2fa1b5860e934307cf7` |
| `goreleaser/goreleaser-action` | `v7.2.2` | `5daf1e915a5f0af01ddbcd89a43b8061ff4f1a89` |

## Publishing Flow

1. Land normal feature, fix, docs, and maintenance commits using Conventional
   Commit prefixes.
2. Let the release workflow open or update the release-please PR.
3. Review the release PR for correct changelog and manifest updates.
4. Merge the release PR.
5. Confirm the release workflow created a `secrets-sync-vX.Y.Z` GitHub release.
6. Confirm GoReleaser uploaded archives and `checksums.txt`.
7. Verify the action can be referenced with:

```yaml
- uses: jbcom/secrets-sync@secrets-sync-vX.Y.Z
  with:
    config: config.yaml
```

## Marketplace Publication

GitHub Marketplace publication is completed from a GitHub release in the UI.
Use the release that release-please created. Do not create a parallel manual
release just to publish the Marketplace listing.

Checklist:

- Repository is public.
- `action.yml` exists at repository root.
- `action.yml` metadata, inputs, branding, and Docker image reference are valid.
- `README.md`, `docs/GITHUB_ACTIONS.md`, and
  `docs/ACTION_QUICK_REFERENCE.md` show the component tag shape.
- Security, privacy, support, contributing, and license documents are present.
- A real release exists for the tag being published.

## Post-Release Verification

```bash
gh release view secrets-sync-vX.Y.Z --repo jbcom/secrets-sync
gh workflow run ci.yml --repo jbcom/secrets-sync
```

Also verify:

- Release assets are present for supported OS and architecture combinations.
- `checksums.txt` is attached.
- Marketplace examples render correctly.
- The latest documentation does not reference old monorepo package tags using
  the `secretssync-v...` shape.

## Manual Repairs

Manual tags are a repair path, not the release process. If a release workflow
fails after release-please creates a tag:

1. Keep the failed tag intact while diagnosing unless the release is proven
   unrecoverable.
2. Prefer rerunning the failed workflow job.
3. If a bad GitHub release was published, delete only the bad release artifacts
   needed for repair.
4. Document the repair in the PR or release notes.

Do not create moving major aliases such as `v1` unless the repository decides
to maintain them deliberately; release-please will not update those aliases by
default.
