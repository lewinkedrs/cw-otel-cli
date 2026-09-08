# Examples

Real-world ways to use `cw-otel-cli`.

| Example | What it shows |
|---------|---------------|
| [github-actions-slo-gate](./github-actions-slo-gate/) | Query a CloudWatch OTel metric from a GitHub Actions workflow and fail the job if it breaches an SLO — a post-deployment gate. Uses OIDC (no stored secrets). |
| [incident-report](./incident-report/) | A single script that runs several queries and prints one consolidated fleet health report — the artifact you run when you get paged. |
| [metrics-to-csv-duckdb](./metrics-to-csv-duckdb/) | Export a per-service metric snapshot to CSV and analyze it with SQL in DuckDB — ranking services and flagging error-budget breaches. |

More examples welcome — see [CONTRIBUTING](../CONTRIBUTING.md).
