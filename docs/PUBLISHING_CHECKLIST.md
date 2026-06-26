# Publishing Checklist

SecretSync releases are automated from `main` with release-please, GoReleaser,
and PyPI trusted publishing for the Python bridge. Do not hand-edit versions,
changelog entries, release tags, or GitHub releases during the normal release
path.

## Release Model

- `release.yml` owns release-please version detection, changelog updates,
  release PRs, and Git tags.
- The root package name is `secrets-sync`, so Go component release tags use the
  `secrets-sync-vX.Y.Z` shape.
- The bridge package name is `secrets-sync-bridge`, so Python component release
  tags use the `secrets-sync-bridge-vX.Y.Z` shape.
- `cd.yml` runs only after release-please reports a Go and/or bridge release.
- GoReleaser builds binary archives and checksums. Container and Marketplace
  publication are separate release surfaces.
- The bridge publish job uses OIDC trusted publishing through `uv publish`; no
  PyPI token should be stored in repository secrets for the normal path.
- The Docker action currently references `docker://jbcom/secrets-sync:v1` from
  `action.yml`; digest pinning should be added only when release automation can
  refresh that digest reliably.

## Maintainer Preflight

Run these before merging a release PR or manually dispatching release workflow
diagnostics:

```bash
go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...
go test ./...
go build -o bin/secrets-sync ./cmd/secrets-sync
tox -e py311,py312,py313,py314,lint,typecheck,docs,build
goreleaser check
docker build -t secrets-sync-test .
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
| `actions/checkout` | `v7.0.0` | `9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0` |
| `actions/setup-go` | `v6.5.0` | `924ae3a1cded613372ab5595356fb5720e22ba16` |
| `actions/setup-python` | `v6.3.0` | `ece7cb06caefa5fff74198d8649806c4678c61a1` |
| `astral-sh/setup-uv` | `v8.2.0` | `fac544c07dec837d0ccb6301d7b5580bf5edae39` |
| `googleapis/release-please-action` | `v5.0.0` | `45996ed1f6d02564a971a2fa1b5860e934307cf7` |
| `goreleaser/goreleaser-action` | `v7.2.2` | `5daf1e915a5f0af01ddbcd89a43b8061ff4f1a89` |

## Publishing Flow

1. Land normal feature, fix, docs, and maintenance commits using Conventional
   Commit prefixes.
2. Let the release workflow open or update the release-please PR.
3. Review the release PR for correct changelog and manifest updates.
4. Merge the release PR.
5. Confirm the release workflow created the expected `secrets-sync-vX.Y.Z`
   and/or `secrets-sync-bridge-vX.Y.Z` GitHub release.
6. Confirm GoReleaser uploaded archives and `checksums.txt` for Go releases.
7. Confirm `cd.yml` published `secrets-sync-bridge` to PyPI for bridge releases.
8. Verify the action can be referenced with:

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
gh release view secrets-sync-bridge-vX.Y.Z --repo jbcom/secrets-sync
gh workflow run ci.yml --repo jbcom/secrets-sync
```

Also verify:

- Release assets are present for supported OS and architecture combinations.
- `checksums.txt` is attached.
- Marketplace examples render correctly.
- The latest documentation does not reference old monorepo package tags using
  the `secrets-sync-v...` shape.

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
