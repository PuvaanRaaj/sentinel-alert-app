# Branch Protection Guidelines

Suggested GitHub branch protection rules for `main`:
- Require pull request reviews (at least 1 approval).
- Require status checks to pass: `test`, `docker` (and any future CI jobs).
- Require signed commits: optional, recommended for regulated environments.
- Disallow force pushes and deletions on `main`.
- Require linear history.

Release tagging:
- Use `v*` tags (e.g., `v0.1.0`) to trigger release builds.

Secrets and tokens:
- Keep `GITHUB_TOKEN` with minimal permissions; use environment-specific secrets for publishing.
