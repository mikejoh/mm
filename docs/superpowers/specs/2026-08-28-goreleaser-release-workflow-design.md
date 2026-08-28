# GoReleaser release workflow

## Problem

`mm` is a pure Go library (`go get github.com/mikejoh/mm`) with no CI-driven
release process — tags are pushed manually with no generated changelog or
GitHub Release. We want tag-triggered GitHub Releases with a GoReleaser-style
changelog, grouped by conventional-commit prefix, without building or
publishing any binary artifact (the library ships no `cmd`; `examples/basic`
is a docs example, not a release target).

Reference implementation: `~/Repos/personal/rke2diff/.goreleaser.yaml` and
`~/Repos/personal/rke2diff/.github/workflows/goreleaser.yml`, adapted for a
library (no builds/archives) instead of a multi-platform binary.

## Design

### `.goreleaser.yaml` (new, repo root)

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

- `builds` is explicitly skipped rather than omitted, so GoReleaser doesn't
  try to auto-detect a main package (it would otherwise find
  `examples/basic/main.go` and attempt to build/release that). `archives`
  is omitted entirely: GoReleaser v2.14's `Archive` config has no `skip`
  field, and with zero builds there's nothing for the archive pipe to
  package anyway (verified via `goreleaser release --clean --skip=validate,publish`).
- `changelog.use: github` pulls commit/PR metadata from the GitHub API
  (available via the workflow's `GITHUB_TOKEN`) for richer changelog entries
  than raw `git log`.
- Groups match this repo's existing commit style (`fix:`, `chore(deps):`,
  etc. — see recent history) into Features / Bug fixes / Other sections.

### `.github/workflows/goreleaser.yml` (new)

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

- Triggers only on `v*` tags (mm has legacy bare tags like `0.1.0` already
  pushed; new releases should stay on the `v`-prefixed scheme already in use
  since `v0.1.2`).
- `go-version: stable`, not `go-version-file: go.mod`, matching this repo's
  existing `go.yml` convention (deliberately decided in commit `130ba9e`,
  unlike rke2diff's workflow which does use `go-version-file`).
- `fetch-depth: 0` is required so GoReleaser can walk full commit history
  since the previous tag for the changelog.
- No build/publish steps beyond GoReleaser itself — the release has no
  downloadable assets, only release notes.

## Testing

1. `goreleaser check` against the new config — validates syntax.
2. Local dry run: `goreleaser release --snapshot --clean --skip=publish` —
   confirms the changelog groups render correctly without needing a real tag
   or network publish.
3. After merging: push a real `vX.Y.Z` tag, confirm the Actions run
   succeeds and produces a correctly grouped, asset-free GitHub Release.

## Out of scope

- Building/publishing binaries, Homebrew taps, Docker images, checksums —
  `mm` ships no binary, so none of this applies.
- Changing the existing `go.yml` PR-check workflow.
