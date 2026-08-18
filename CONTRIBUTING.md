# Contributing

Thanks for your interest in `cwpromql`!

## Development

```bash
# Corp networks often block the default Go module proxy:
go env -w GOPROXY=direct GOSUMDB=off

make build      # build ./bin/cwpromql
make test       # unit tests
make cover      # tests + coverage summary
make vet        # go vet
make fmt        # gofmt -w .
make all        # fmt-check + vet + test + build
```

There is also a **live smoke test** that hits a real CloudWatch account (needs
AWS credentials with `cloudwatch:GetMetricData` + `cloudwatch:ListMetrics`):

```bash
make live       # CWPROMQL_LIVE=1 go test ./internal/promql -run TestLive -v
```

## Guidelines

- Run `make all` before opening a PR; CI enforces gofmt, `go vet`, build, and
  `go test -race`.
- Add or update tests for behavior changes. Network calls are tested with
  `httptest` (see `internal/promql/client_test.go`) — no AWS account required.
- Keep the CLI surface documented in `README.md` and, where agent-relevant, in
  `SKILL.md`.
- Prefer small, focused PRs with a clear description of what and why.

## Reporting issues

Open a GitHub issue with the command you ran, what you expected, what happened
(include `--output json` where useful), and your `cwpromql --version`.
