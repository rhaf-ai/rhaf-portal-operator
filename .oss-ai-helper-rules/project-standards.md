# Project Standards

Local OSS Helper rules for RHAF Portal Operator.

- Build tool: Go (Make)
- Build command: `make build`
- Test command: `make test`
- Test with coverage command: `go test -cover ./...`
- Format command: `make fmt`
- Module-specific build: no
- Parallelized Maven: n/a
- Code style restrictions: Follow standard Go conventions, `.golangci.yml`, and `AGENTS.md`; run `make fmt-check`, `make test`, and `make check`.

## Version

91e4f414a232406e41c2fa09af42f77b833f2d69
