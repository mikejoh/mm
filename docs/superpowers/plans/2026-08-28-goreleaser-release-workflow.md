# GoReleaser Release Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tag-push (`vX.Y.Z`) triggers a GitHub Actions workflow that runs GoReleaser to publish a GitHub Release with a changelog grouped by commit type (Features / Bug fixes / Other) — no binary artifacts, since `mm` is a library.

**Architecture:** Two new files, no code changes. `.goreleaser.yaml` configures GoReleaser to skip builds/archives and only emit a grouped changelog + GitHub Release. `.github/workflows/goreleaser.yml` runs `goreleaser release --clean` on `v*` tag pushes, reusing this repo's existing action versions and Go setup convention.

**Tech Stack:** GoReleaser v2, GitHub Actions (`actions/checkout@v7`, `actions/setup-go@v7`, `goreleaser/goreleaser-action@v7`).

**Spec:** `docs/superpowers/specs/2026-08-28-goreleaser-release-workflow-design.md`

## Global Constraints

- `builds` and `archives` must both be explicitly `skip: true` — never omitted — so GoReleaser doesn't auto-detect `examples/basic/main.go` as a release target.
- Release trigger is `v*` tags only (not bare tags like the legacy `0.1.0`).
- Go setup in the new workflow uses `go-version: stable`, matching `.github/workflows/go.yml`'s existing convention — not `go-version-file: go.mod`.
- No build/publish/checksum steps: the release must have zero downloadable assets, release notes only.

---

### Task 1: Add and validate `.goreleaser.yaml`

**Files:**
- Create: `.goreleaser.yaml` (repo root)

**Interfaces:**
- Produces: a GoReleaser config consumed by the `goreleaser/goreleaser-action@v7` step added in Task 2 (via `args: release --clean`).

- [x] **Step 1: Write `.goreleaser.yaml`**

```yaml
version: 2
project_name: mm
release:
  github:
    owner: mikejoh
    name: mm
  name_template: 'v{{ .Tag }}'
builds:
  - skip: true
changelog:
  use: github
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\(.+\))??!?:.+$'
      order: 0
    - title: Bug fixes
      regexp: '^.*?fix(\(.+\))??!?:.+$'
      order: 1
    - title: Other
      order: 999
```

Deviation from the original plan: no `archives: [{skip: true}]` block.
GoReleaser v2.14's `Archive` config has no `skip` field (`goreleaser check`
fails with `field skip not found in type config.Archive`), and with
`builds` skipped there's nothing for the archive pipe to package anyway —
confirmed no error with `archives` omitted entirely.

- [x] **Step 2: Validate config syntax**

Run: `goreleaser check`
Actual output:
```
  • checking                                  path=.goreleaser.yaml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
```

- [x] **Step 3: Dry-run the release to inspect the changelog**

`--snapshot` mode skips changelog generation entirely (it's tied to a real
tag comparison), so the dry run instead used:
`GITHUB_TOKEN=$(gh auth token) goreleaser release --clean --skip=validate,publish,announce`
— this renders the real changelog (comparing the two most recent existing
tags, v0.2.0..v0.3.0) without publishing anything. Result in
`dist/CHANGELOG.md`:
```
## Changelog
### Bug fixes
* 130ba9e...: fix: bump golangci-lint-action to v9 for golangci-lint v2 support (@mikejoh)
* 833debf...: fix: pin golangci-lint to latest and scope to new issues only (@mikejoh)
* be9871e...: fix: prevent duplicate metric registration panics, add tests and LICENSE (@mikejoh)
* 939ec2d...: fix: use stable Go toolchain in CI instead of go.mod's minimum version (@mikejoh)
### Other
* 1ffbdc4...: Add CI workflow, dependabot config, and CI badge (@mikejoh)
* 43bde7c...: Merge pull request #1 from mikejoh/chore/standardize-ci-tooling (@mikejoh)
* aa4ed0b...: Merge pull request #4 from mikejoh/chore/golangci-lint-action-v9 (@mikejoh)
* 9fa9117...: chore(deps): bump github.com/prometheus/client_golang to v1.19.1 (@mikejoh)
```
Grouping confirmed correct: `fix:` commits under Bug fixes, everything else
under Other. `dist/` removed afterward (dry-run output, not committed).

- [x] **Step 4: Commit**

```bash
git add .goreleaser.yaml
git commit -m "ci: add goreleaser config for grouped-changelog releases" --trailer "Assisted-by: Claude Code/claude-sonnet-5"
```

### Task 2: Add the release workflow

**Files:**
- Create: `.github/workflows/goreleaser.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` from Task 1 (implicitly, via `goreleaser release --clean` reading the repo-root config).

- [x] **Step 1: Write `.github/workflows/goreleaser.yml`**

```yaml
name: goreleaser

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v7
        with:
          fetch-depth: 0
      - name: Set up Go
        uses: actions/setup-go@v7
        with:
          go-version: stable
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [x] **Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/goreleaser.yml'))" && echo OK`
Actual: `OK`

- [x] **Step 3: Commit**

```bash
git add .github/workflows/goreleaser.yml
git commit -m "ci: add tag-triggered goreleaser release workflow" --trailer "Assisted-by: Claude Code/claude-sonnet-5"
```

---

## Manual verification (not automated — do this yourself when ready)

Pushing a real tag creates a real, visible GitHub Release, so it's left as a manual step rather than a plan task:

1. Push the two commits above to `main` (`git push`).
2. When ready to cut an actual release, tag it and push the tag, e.g.:
   ```bash
   git tag v0.4.0
   git push origin v0.4.0
   ```
3. Watch the "goreleaser" workflow run in the Actions tab.
4. Confirm the resulting GitHub Release at `github.com/mikejoh/mm/releases/tag/v0.4.0` has a changelog grouped into Features/Bug fixes/Other and no attached binary assets.
