# Publishing Checklist

SecretSync releases are automated from `main` with release-please and
GoReleaser. Do not hand-edit versions, changelog entries, release tags, or
GitHub releases during the normal release path.

## Release Model

- `release.yml` owns release-please version detection, changelog updates,
  release PRs, and Git tags.
- Release-please uses plain semver tags in the `vX.Y.Z` shape.
- `cd.yml` runs only after release-please reports a Go release.
- GoReleaser builds CLI binary archives, Kubernetes controller archives, Lambda
  archives, and checksums.
- `cd.yml` publishes the GHCR image `ghcr.io/jbcom/secrets-sync` for direct
  container use, Kubernetes controller/CronJob use, and for the Docker-based
  GitHub Action. The image runtime is Google Distroless static.
- `cd.yml` also builds, repairs, and publishes the gopy binding distribution
  `secrets-sync-python-binding` through PyPI trusted publishing. Linux wheels
  must be repaired to manylinux tags before upload; raw `linux_*` platform tags
  are a release blocker.
- The Docker action currently references `docker://ghcr.io/jbcom/secrets-sync:v1`
  from `action.yml`; digest pinning should be added only when release automation
  can refresh that digest reliably.

## Maintainer Preflight

Run these before merging a release PR or manually dispatching release workflow
diagnostics:

```bash
just vuln
just test-go
just build-all
just python-matrix
just quality
just goreleaser-check
just docker-build secrets-sync-test
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
| `actions/configure-pages` | `v6.0.0` | `45bfe0192ca1faeb007ade9deae92b16b8254a0d` |
| `actions/upload-pages-artifact` | `v5.0.0` | `fc324d3547104276b827a68afc52ff2a11cc49c9` |
| `actions/deploy-pages` | `v5.0.0` | `cd2ce8fcbc39b97be8ca5fce6e763baed58fa128` |
| `actions/upload-artifact` | `v5.0.0` | `330a01c490aca151604b8cf639adc76d48f6c5d4` |
| `actions/download-artifact` | `v8.0.1` | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` |

## Publishing Flow

1. Land normal feature, fix, docs, and maintenance commits using Conventional
   Commit prefixes.
2. Let the release workflow open or update the release-please PR.
3. Review the release PR for correct changelog and manifest updates.
4. Merge the release PR.
5. Confirm the release workflow created the expected `vX.Y.Z` GitHub release.
6. Confirm GoReleaser uploaded CLI archives, controller archives, Lambda archives, and
   `checksums.txt`.
7. Confirm GHCR shows `ghcr.io/jbcom/secrets-sync` tags for the component
   release tag, semver tag, major tag, and `latest`.
8. Confirm PyPI shows `secrets-sync-python-binding` for the same release and
   the uploaded Linux files use manylinux tags, not raw `linux_*` tags.
9. Verify the action can be referenced with:

```yaml
- uses: jbcom/secrets-sync@vX.Y.Z
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
  `docs/ACTION_QUICK_REFERENCE.md` show the plain semver tag shape.
- Security, privacy, support, contributing, and license documents are present.
- A real release exists for the tag being published.

## Post-Release Verification

```bash
gh release view vX.Y.Z --repo jbcom/secrets-sync
gh workflow run ci.yml --repo jbcom/secrets-sync
```

Also verify:

- Release assets are present for supported OS and architecture combinations.
- Controller archives are present for supported OS and architecture
  combinations and include `deploy/crds` plus `deploy/controller` manifests.
- Lambda archives are present for Linux AMD64 and ARM64.
- The GHCR image tag pulls successfully.
- `checksums.txt` is attached.
- Marketplace examples render correctly.
- The latest documentation does not reference old monorepo package tags using
  the `secrets-sync-v...` shape.

## Manual Repairs

Manual tags are a repair path, not the release process. If a release workflow
fails after release-please creates a tag:

1. Keep the failed tag intact while diagnosing unless the release is proven
   unrecoverable.
2. Prefer rerunning the failed workflow job. The CD workflow removes existing
   release assets for the same tag before rerunning GoReleaser so a partial
   release can be replaced without deleting or moving the tag.
3. If a bad GitHub release was published outside the workflow, delete only the
   bad release artifacts needed for repair.
4. Document the repair in the PR or release notes.

Do not create moving major aliases such as `v1` unless the repository decides
to maintain them deliberately; release-please will not update those aliases by
default.
