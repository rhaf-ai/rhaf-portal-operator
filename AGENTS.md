# RHAF Portal Operator — AI Agent Guidelines

These guidelines adapt the [Camel K baseline](https://github.com/apache/camel-k/blob/main/AGENTS.md)
and the conventions used by [Wanaku](https://github.com/wanaku-ai/wanaku/blob/main/AGENTS.md).

## Project info

RHAF Portal Operator is the Go operator for RHAF Portal Core on Kubernetes and
OpenShift. The current scaffold prints a greeting and exits. It has no CRDs,
controllers, health endpoints, or cluster installation assets yet.

- Module: `github.com/rhaf-ai/rhaf-portal-operator`.
- Go version: read `go.mod`.
- Build entry point: `Makefile`.
- Executable: `cmd/rhaf-portal-operator/main.go`.
- Issue tracker: GitHub Issues in `rhaf-ai/rhaf-portal-operator`.
- Core runtime: `rhaf-ai/rhaf-portal-core`.

This repository owns the Go operator. Core owns the Rust runtime. Keep these
responsibilities separate.

## Core guidelines

Think before you write. Use the minimum code required. Do not add abstractions
without a concrete need. Do not bypass lint rules to make a check pass.

Do not publish private repository names, local design paths, links, or planning
references without explicit authorization.

Use OSS Helper project rules when available. Prefer the applicable Ripwire skill
for navigation, reuse, and change checks. The `.oss-ai-helper-rules/` directory
requires manual maintainer review. Keep it uncommitted until explicitly approved.

## Build and quality

```bash
make build           # build/rhaf-portal-operator
make run             # run the greeting
make test            # Go tests and go vet
make fmt             # gofmt and goimports
make fmt-check       # reject formatting changes
make lint            # golangci-lint
make check           # lint and govulncheck
make build-image     # native Linux image, Podman by default
```

Quality tools are pinned in `Makefile` and installed in `bin/` on first use.
Do not parallelize resource-intensive build and test jobs. If a required tool is
missing, run `notify-pushover "Needs to install <tool name>"` and inform the user.

Follow standard Go conventions and `.golangci.yml`. Keep public Go APIs and CRD
fields compatible when they exist. Document deprecations and migration steps.
When code generation is introduced, commit generated CRDs, RBAC, clients, and
deepcopy code with the source changes. Do not claim these generators exist today.

## Tests and acceptance

- Add meaningful tests for new behavior and bug fixes.
- Do not add tests that only repeat the hello-world implementation.
- Update documentation with behavior and configuration changes.
- Run formatting, relevant tests, lint, and vulnerability checks before submission.
- Use bounded polling for asynchronous tests. Do not use `time.Sleep` to await state.
- Report checks that could not run and their concrete blockers.

Cluster and end-to-end test infrastructure is deferred. Do not imply a local build
validates Kubernetes reconciliation or OpenShift deployment.

## Containers and CI

Keep `Containerfile`, `Makefile`, and `hack/` scripts consistent when source layout
changes. Runtime images use a non-root UID and root group for OpenShift support.
Do not add ports or health checks before the process provides those endpoints.

CI builds Linux amd64 and arm64 images. Manifest publication combines their tags.
Releases also include macOS amd64 and arm64 archives. Registry settings come from
`CONTAINER_REGISTRY` and `CONTAINER_GROUP`; credentials use `QUAY_USERNAME` and
`QUAY_PASSWORD`. Do not put credentials in files or logs.

## Investigation and review

Before fixing an issue, confirm the failure and read relevant source, history,
design decisions, and related issues. Check whether a proposed fix reverses an
intentional design choice. Explain any material conflict before implementation.
Verify external versions and API behavior against official sources when uncertain.

Review changes to secrets, RBAC, cross-namespace access, endpoint registration,
and external fetches for changes to trust boundaries. Keep permissions minimal.
Report suspected vulnerabilities privately to the maintainer; do not publish
exploit details in a public issue or PR.

## Git and pull requests

- Create a branch for new issue work: `ci-issue-<ISSUE_NUMBER>`.
- Use a descriptive branch when no issue number exists.
- Do not push to a branch created by another contributor without permission.
- Do not reassign a human's issue. Check ownership before taking an unassigned issue.
- Use one commit per logical change. Prefer GPG signatures; unsigned fallback is allowed if signing fails.
- Use `git mv` for tracked files. Preserve other contributors' edits.
- Use concise commit messages: `Fix #<ISSUE_NUMBER>: ...`, `chore: ...`, or `ci: ...`.
- Use short, professional PR titles and descriptions. Include relevant validation.
- Create PRs with `gh pr create --title "..." --body "..."`.
- Keep the PR title and description current after changes. Link the issue when applicable.
- Identify AI-generated GitHub content and name the human operator.
- Request relevant human reviewers. Re-request review after addressing feedback.
- Do not merge with unresolved conversations or without human approval.
- Do not approve your own PR. Limit agents to 10 PRs per operator per day.

## Documentation

Use ASD-STE100 Simplified Technical English for procedures, commands,
configuration, and accuracy-critical behavior. Use natural technical English for
introductions and navigation. Use short, active sentences and consistent terms.
Preserve commands, URLs, identifiers, product names, and technical meaning.
