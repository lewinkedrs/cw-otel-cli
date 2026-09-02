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

## Set up GitHub OIDC (one-time)

The workflow authenticates to AWS with **GitHub OIDC** — short-lived, per-run
credentials with no long-lived access keys stored in the repo. You set this up
once per AWS account. AWS's official walkthrough:
[Configuring OpenID Connect in AWS](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services).

The short version:

1. **Create the GitHub OIDC identity provider** in IAM (only needed once per
   account):
   - Provider URL: `https://token.actions.githubusercontent.com`
   - Audience: `sts.amazonaws.com`

2. **Create a role** the workflow assumes, with a trust policy scoped to your
   repo (so only your repo's workflows can assume it):

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{
       "Effect": "Allow",
       "Principal": {
         "Federated": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
       },
       "Action": "sts:AssumeRoleWithWebIdentity",
       "Condition": {
         "StringEquals": {
           "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
         },
         "StringLike": {
           "token.actions.githubusercontent.com:sub": "repo:YOUR_ORG/YOUR_REPO:*"
         }
       }
     }]
   }
   ```

3. **Attach a least-privilege permissions policy** — the CLI needs only these
   two actions:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{
       "Effect": "Allow",
       "Action": ["cloudwatch:GetMetricData", "cloudwatch:ListMetrics"],
       "Resource": "*"
     }]
   }
   ```

4. Put the role's ARN in `role-to-assume` in [`slo-gate.yml`](./slo-gate.yml),
   and keep `permissions: id-token: write` in the workflow.

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
