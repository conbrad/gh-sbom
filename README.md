# gh-sbom

A [GitHub CLI](https://cli.github.com) extension that exports the SBOM (software bill of materials) for every repo in an org and aggregates the results into one combined package list.

It uses GitHub's native dependency-graph SBOM endpoint ([`GET /repos/{owner}/{repo}/dependency-graph/sbom`](https://docs.github.com/en/rest/dependency-graph/sboms)), which returns the same server-side-computed data shown in a repo's
**Insights → Dependency graph** tab — so there's no cloning and no local scanning. One flat REST call per repo.

It's a precompiled Go extension built on [go-gh](https://github.com/cli/go-gh), so it reuses your existing `gh` auth and has no runtime dependencies.

## Install

```sh
gh extension install conbrad/gh-sbom
```

Or from a local checkout:

```sh
go build && gh extension install .
```

## Usage

```sh
# every repo in an org (or user account)
gh sbom my-org

# a single repo
gh sbom cli/cli

# tune outputs
gh sbom my-org --output sboms --format csv --out deps.csv --top 50

# re-run the aggregation/rollup over already-downloaded SBOMs (no API calls)
gh sbom --skip-fetch
```

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-o, --output <dir>` | `sboms` | Directory for raw SPDX JSON, one file per repo |
| `-f, --format <fmt>` | `tsv` | Combined table format: `tsv`, `csv`, `json`, or `html` |
| `--out <file>` | `combined.<format>` | Combined table output path |
| `-l, --limit <n>` | `1000` | Max repos to list from the org |
| `-n, --top <n>` | `20` | Rows in the "most common packages" rollup |
| `--include-archived` | off | Include archived repos |
| `--skip-fetch` | off | Re-aggregate existing JSON only |

## Output

- `sboms/<repo>.json`: the raw SPDX 2.3 SBOM for each repo
- `combined.<format>`: the combined dependency table (`--format tsv|csv|json|html`), one entry per dependency. TSV/CSV columns and JSON object keys: `repo`, `ecosystem`, `package`, `version`:

  ```json
  [{"repo": "cli", "ecosystem": "golang", "package": "github.com/microcosm-cc/bluemonday", "version": "v1.0.27"}]
  ```

  `--format html` produces a single self-contained, styled document (no JS, no external assets) — open it in a browser or attach it to a PR/issue for a quick shareable view. It's for reading, not parsing; use tsv/csv/json for that.

The ecosystem column is derived from each package's [purl](https://github.com/package-url/purl-spec) (`pkg:golang/...` →`golang`); the repo's own root package is excluded so rollups only count real dependencies.

A summary and a "top packages by repo count" rollup print at the end. The output table composes with standard tools for anything else:

```sh
# unique packages across the whole org
cut -f3 combined.tsv | tail -n +2 | sort -u

# most common dependency org-wide (by total entries, incl. multiple versions)
cut -f3 combined.tsv | tail -n +2 | sort | uniq -c | sort -rn | head -30

# everything on a specific package, org-wide
grep -P '\tlodash\t' combined.tsv

# with --format json: repos depending on a given package
jq -r '.[] | select(.package == "lodash") | .repo' combined.json
```

## Rate limits

Each repo costs one request against the standard 5,000 requests/hour authenticated REST limit, so even large orgs fit comfortably. The tool checks your remaining quota up front and warns if it looks tight:

```sh
gh api rate_limit --jq '.resources.core'
```

## Caveats

- **Dependency graph must be enabled** for a repo. It's on by default for public repos; for private repos an org admin may need to enable it under **Settings → Advanced Security → Dependency graph**. Disabled repos are reported and skipped.
- Results reflect **declared dependencies** that GitHub's parsers understand (npm, pip, Maven, Cargo, Go modules, RubyGems, NuGet, Composer, GitHub Actions, etc.) — including transitive dependencies from lockfiles. It is not a from-scratch scan like syft, so it won't catch OS-level packages baked into a Dockerfile.
- TSV/CSV output is RFC 4180: a field containing the delimiter, a double quote, a newline, or leading whitespace is quoted. Real package names are virtually never like that, but the naive `cut`/`grep` recipes above would mis-read such a row — use `--format json` + `jq` when you need bulletproof parsing.
- During aggregation, `*.json` files in the output directory that aren't SBOMs (for example the tool's own `--format json` table, if you pointed `--out` there) are skipped with a warning.

## Releasing

Push a tag like `v0.1.0` and the `release` workflow
([`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile))
cross-compiles binaries for every platform and attaches them to a GitHub
release, which is what `gh extension install` downloads.
