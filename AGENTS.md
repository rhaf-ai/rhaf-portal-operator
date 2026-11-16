# RHAF Portal Operator — AI Agent Guidelines

These guidelines adapt the [Camel K baseline](https://github.com/apache/camel-k/blob/main/AGENTS.md)
and the conventions used by [Wanaku](https://github.com/wanaku-ai/wanaku/blob/main/AGENTS.md).

## Project info

RHAF Portal Operator is the Go operator for RHAF Portal Core on Kubernetes and
OpenShift. It provides namespaced `RHAFPortalCore` and `RHAFMCPServer` CRDs,
their generated RBAC, finalizer-only controllers, and health endpoints. Workload
and endpoint-registration reconciliation follow in later issues.

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
make test            # Go tests and go vet
make fmt             # gofmt and goimports
make fmt-check       # reject formatting changes
make lint            # golangci-lint
make check           # lint and govulncheck
make build-image     # build a Linux image for a Kubernetes or OpenShift cluster
make dev-minikube    # build, deploy, and run in Minikube
make dev-openshift-local # build, deploy, and run in OpenShift Local
```

Quality tools are pinned in `Makefile` and installed in `bin/` on first use.
Do not parallelize resource-intensive build and test jobs. If a required tool is
missing, run `notify-pushover "Needs to install <tool name>"` and inform the user.

Follow standard Go conventions and `.golangci.yml`. Format code and pass
`make fmt-check`, `make test`, and `make check` before pushing code changes.
Keep public Go APIs and CRD field semantics compatible when they exist. Justify
incompatible changes and provide upgrade notes. Mark deprecated Go fields and
APIs with `// Deprecated:` comments. Update the relevant schemas and documentation
with deprecation and migration guidance.

### Generated artifacts

Commit generated CRDs, RBAC, clients, and deepcopy code with the source changes.
Change the source and run `make generate manifests`; do not hand-edit generated
artifacts. Regenerate documentation derived from code when its source changes.
Check for uncommitted generated changes after a build.

## Tests and acceptance

- Add meaningful tests for new behavior and bug fixes.
- Do not add tests that only repeat the hello-world implementation.
- Update documentation with behavior and configuration changes.
- Run formatting, relevant tests, lint, and vulnerability checks before submission.
- Report checks that could not run and their concrete blockers.

Cluster and end-to-end test infrastructure is deferred. Do not imply a local build
validates Kubernetes reconciliation or OpenShift deployment.

### Asynchronous Testing: Use `Eventually`, not `time.Sleep`

Do not use `time.Sleep()` to wait for asynchronous state in tests. Fixed sleeps
make tests slow, flaky, and dependent on timing. Poll for the expected condition
with a timeout. Use Gomega's `Eventually` in suites that use Gomega, or a bounded
poll helper in unit tests.

- New tests must not introduce `time.Sleep()` to await state.
- When modifying an existing test that sleeps to await state, migrate that wait
  to `Eventually` or a bounded poll helper.
- Always set an explicit timeout so a failed condition cannot hang the suite.
- Use a clear assertion or predicate for the expected state.
- Do not replace a sleep with a busy-wait loop.

## Containers and CI

Keep `Containerfile`, `Makefile`, and `scripts/` consistent when source layout
changes. Runtime images use a non-root UID and root group for OpenShift support.
Do not add ports or health checks before the process provides those endpoints.
Use Minikube or OpenShift Local to build, deploy, and run the operator during
development. The operator requires in-cluster credentials from its service
account and does not run as a standalone local container.

CI builds Linux amd64 and arm64 images. Manifest publication combines their tags.
Releases also include macOS amd64 and arm64 archives. Registry settings come from
`CONTAINER_REGISTRY` and `CONTAINER_GROUP`; credentials use `QUAY_USERNAME` and
`QUAY_PASSWORD`. Do not put credentials in files or logs.

## Investigation and review

### Issue investigation before implementation

Do not implement a fix based only on an issue description or code that appears
wrong. Confirm the failure and understand why the current behavior exists.

1. Reproduce the problem. Check the report's assumptions and expected behavior.
2. Run `git log --oneline -- <file>` and `git blame <file>` on affected code.
   Read the commits and linked issues that explain the current implementation.
3. Check related issues, PRs, and repository documentation for prior decisions,
   rejected approaches, and compatibility constraints.
4. Check whether the proposed fix reverses an intentional design choice. If it
   does, explain why the reversal is justified in the PR description.
5. Explain material risks, ambiguities, or conflicts before implementation.

### Knowledge cutoff awareness

Do not make authoritative claims about external projects from training knowledge
alone. Verify referenced dependency versions and relevant API behavior against
official sources before relying on or questioning them. State uncertainty when
verification is not possible.

### Git history review

Apply the same investigation when reviewing PRs. Read relevant history, blame,
documentation, and related issues. Check that a proposed fix does not silently
reverse a deliberate behavior or break a public contract.

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
