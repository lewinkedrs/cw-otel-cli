# GitHub Actions: post-deployment SLO gate

Query a CloudWatch OTel metric from CI and **fail the workflow if it breaches a
threshold** — so a bad deploy blocks promotion.

## Why this needs `cw-otel-cli`

The metric lives behind CloudWatch's Prometheus-compatible **PromQL API**, which
is SigV4-signed under the `monitoring` service and has **no `aws-cli` equivalent**.
Querying it from a shell otherwise means hand-writing SigV4 signing or pulling in
a heavy SDK. `cw-otel-cli` is a single static binary: download it, run one query,
parse the JSON.

## Files

| File | Purpose |
|------|---------|
| [`slo-gate.yml`](./slo-gate.yml) | A complete, copy-pasteable workflow. Copy it into `.github/workflows/` and adapt. |
| [`check-slo.sh`](./check-slo.sh) | The query + threshold logic, extracted so you can run it locally with the same binary. |

## How it works

1. **Auth** — `aws-actions/configure-aws-credentials@v4` assumes a role via
   GitHub OIDC (`permissions: id-token: write`). No stored access keys.
2. **Install** — downloads the pinned `cwpromql_<version>_Linux_x86_64.tar.gz`
   from the [releases page](https://github.com/lewinkedrs/cw-otel-cli/releases).
3. **Query + gate** — runs an instant PromQL query, reads the scalar value out of
   the JSON, and exits non-zero if it breaches the threshold, which fails the job.

## Adapt it to your account

Replace these before running:

- `AWS_REGION` — your region.
- `role-to-assume` — your OIDC role ARN. It needs only two IAM actions:
  `cloudwatch:GetMetricData` and `cloudwatch:ListMetrics`.
- `SLO_QUERY` — a PromQL expression that returns a **single scalar series**
  (e.g. an error ratio). CloudWatch requires an exact metric name in the
  selector: use `{__name__="my_metric", label="value"}`, not a bare label
  selector.
- `THRESHOLD` / `COMPARISON` — the numeric breach threshold and direction
  (`gt` fails when the value is above the threshold, `lt` when below).

## Run the gate locally

The gate logic isn't tied to CI — you can run it anywhere the binary and your
AWS credentials are available:

```bash
export AWS_REGION=us-east-1
export SLO_QUERY='sum(rate({__name__="http_requests_total", outcome="error"}[5m])) / sum(rate({__name__="http_requests_total"}[5m]))'
export THRESHOLD=0.02
export COMPARISON=gt
./check-slo.sh
```

## Behavior notes

- **Fails closed**: if the query returns no data, the gate treats it as
  "cannot verify the SLO" and fails — it does not silently pass.
- Requires `jq` (preinstalled on GitHub-hosted runners).
