# kubeOptimum

A Go CLI tool that analyses your Kubernetes cluster, Prometheus metrics, and AWS EKS nodegroups to produce actionable cost-optimisation recommendations.

---

## What it does

kubeOptimum connects to three data sources simultaneously:

| Source | What it reads |
|--------|--------------|
| Kubernetes API | Deployments, Pods, Nodes — resource requests, limits, labels |
| Prometheus | p95 CPU and memory usage per container over a configurable lookback window |
| AWS EKS / EC2 | Nodegroups, instance types, on-demand pricing |

It then runs a set of **advisor checks** and outputs a prioritised list of recommendations such as:

- **Right-size Deployments** — CPU/memory requests are 10× higher than actual p95 usage
- **Idle workloads** — Deployments with near-zero CPU over the last 7 days
- **Nodegroup headroom** — EKS nodegroup has 60 %+ free capacity; downsize or consolidate
- **Instance type swap** — Switch from `m5.2xlarge` to `m5.xlarge` for the same workload
- **Spot opportunities** — Stateless Deployments that could move to Spot and cut costs by ~70 %

Each recommendation includes an estimated monthly saving in USD where pricing data is available.

---

## Prerequisites

- Go 1.26+
- A kubeconfig pointing at your target cluster (or in-cluster service account)
- Prometheus reachable over HTTP (with `node_exporter` and `kube-state-metrics` installed)
- AWS credentials configured (`~/.aws/credentials`, env vars, or IAM role) for EKS clusters
- The AWS Pricing API requires credentials in `us-east-1` regardless of cluster region

---

## Installation

```bash
go install github.com/millicano106/kubeOptimum@latest
```

Or build from source:

```bash
git clone https://github.com/millicano106/kubeOptimum
cd kubeOptimum
make install
```

---

## Quick start

```bash
# Initialise a config file at ~/.kubeOptimum/config.yaml
kubeOptimum config init

# Edit the config to point at your Prometheus URL and EKS cluster name
kubeOptimum config view

# Run the full analysis (defaults: 7d lookback, all namespaces, table output)
kubeOptimum analyze

# Narrow to a single namespace with JSON output
kubeOptimum analyze --namespace production --output json

# Save a timestamped report file
kubeOptimum report --output-file ./reports/
```

---

## Commands

### `kubeOptimum analyze`

Runs all advisor checks and prints recommendations to stdout.

| Flag | Default | Description |
|------|---------|-------------|
| `--kubeconfig` | `$KUBECONFIG` / `~/.kube/config` | Path to kubeconfig |
| `--context` | current context | kubeconfig context to use |
| `--namespace` | all | Kubernetes namespace filter |
| `--window` | `7d` | Prometheus lookback window (e.g. `24h`, `3d`, `14d`) |
| `--output` | `table` | Output format: `table`, `json`, `yaml` |
| `--filter-category` | all | Comma-separated list of categories to show |
| `--filter-severity` | all | `HIGH`, `MEDIUM`, `LOW` |
| `--dry-run` | false | Skip AWS Pricing API (useful without billing access) |

### `kubeOptimum report`

Same as `analyze` but writes output to a timestamped file (e.g. `kubeOptimum-report-2026-05-21.json`).

| Flag | Default | Description |
|------|---------|-------------|
| `--output-file` | `./` | Directory or full path for the report file |
| *(all analyze flags)* | | |

### `kubeOptimum config init`

Writes a default config file to `~/.kubeOptimum/config.yaml`. Safe to run multiple times — does not overwrite an existing file.

### `kubeOptimum config view`

Prints the resolved config (file + env vars + flag overrides) as YAML.

---

## Configuration

Config file location: `~/.kubeOptimum/config.yaml`

All values can be overridden by environment variables prefixed with `KUBEOPTIMUM_` (e.g. `KUBEOPTIMUM_PROMETHEUS_URL`).

```yaml
kubernetes:
  kubeconfig: ""        # leave empty to use $KUBECONFIG or ~/.kube/config
  context: ""           # leave empty to use current context

prometheus:
  url: "http://localhost:9090"
  token: ""             # bearer token for authenticated Prometheus
  username: ""          # basic auth (alternative to token)
  password: ""

aws:
  region: "us-east-1"
  profile: ""           # AWS profile name (leave empty for default)
  roleARN: ""           # assume this IAM role before making API calls
  clusterName: ""       # EKS cluster name (used to filter nodegroups)

analysis:
  lookbackWindow: "7d"
  namespace: ""         # empty = all namespaces
  outputFormat: "table" # table | json | yaml
  thresholds:
    cpuRequestBufferPct: 20    # recommend new request = p95 usage + this % buffer
    memRequestBufferPct: 20
    idleCPUThresholdMilli: 5   # flag as idle if p95 CPU < 5m cores
```

---

## Recommendation output

### Table (default)

```
SEVERITY  CATEGORY       RESOURCE                           SUMMARY                                        EST. SAVING/MO
HIGH      RIGHT_SIZING   deploy/payments-api (default)      CPU request 500m but p95 usage is 12m          ~$18.40
HIGH      IDLE_WORKLOAD  deploy/legacy-reporter (staging)   Max CPU 2m over 7d, consider scaling to zero   ~$34.20
MEDIUM    NODEGROUP      eks/nodegroup-general-v2            67% headroom, downsize from m5.2xl to m5.xl    ~$210.00
LOW       SPOT           deploy/batch-processor (jobs)       No PDB, tolerates interruption, move to Spot   ~$95.00
```

### JSON (`--output json`)

```json
[
  {
    "id": "a3f9b1c2",
    "category": "RIGHT_SIZING",
    "severity": "HIGH",
    "resource": { "kind": "Deployment", "namespace": "default", "name": "payments-api" },
    "summary": "CPU request 500m but p95 usage is 12m",
    "detail": "Over the past 7 days the p95 CPU for all containers was 12m. Current request is 500m. Reducing to 15m (12m + 20% buffer) frees scheduling capacity.",
    "estimatedMonthlySavingUSD": 18.40,
    "currentState": { "cpu_request": "500m", "p95_cpu_usage": "12m" },
    "suggestedAction": { "cpu_request": "15m" },
    "checkName": "rightsizing"
  }
]
```

---

## Prometheus requirements

kubeOptimum queries Prometheus directly. Your Prometheus instance must scrape:

- `container_cpu_usage_seconds_total` — from **cAdvisor** (part of kubelet)
- `container_memory_working_set_bytes` — from cAdvisor
- `node_cpu_seconds_total`, `node_memory_MemAvailable_bytes` — from **node_exporter**
- `kube_pod_container_resource_requests` — from **kube-state-metrics**
- `container_cpu_cfs_throttled_seconds_total` — from cAdvisor (for throttling check)

These are available by default in any standard kube-prometheus-stack deployment.

---

## Architecture

```
cmd/
  root.go         cobra root, persistent flags, viper config load
  analyze.go      analyze subcommand
  report.go       report subcommand
  config.go       config init|view

internal/
  config/         Config struct, Load(), Validate()
  k8s/            Kubernetes client and typed fetch functions
  prometheus/     Prometheus HTTP API client and PromQL wrappers
  cloudprovider/  CloudProvider interface + AWS EKS/EC2/Pricing implementation
  advisor/
    advisor.go    Advisor orchestrator; Check interface
    context.go    AnalysisContext — aggregated data snapshot passed to every check
    recommendation.go  Recommendation, Severity, Category types
    checks/       One file per advisor check (rightsizing, idle, nodegroup, ...)
  output/         Renderer interface + table/json/yaml implementations

pkg/
  units/          CPU millicore <-> float64, bytes <-> human-readable
  timeutil/       Duration parsing for --window flag
```

The **advisor engine** is designed for extensibility: every check implements a single `Check` interface:

```go
type Check interface {
    Name() string
    Description() string
    Run(ctx context.Context, ac *AnalysisContext) ([]Recommendation, error)
}
```

Adding a new check means writing one file in `internal/advisor/checks/` and appending it to the slice in `NewAdvisor()`.

The **cloud provider** is abstracted behind a `CloudProvider` interface so GKE and AKS support can be added without touching the advisor logic.

---

## Development

```bash
# Build
make build

# Run tests (no live cluster, Prometheus, or AWS access needed)
make test

# Lint
make lint

# Install to $GOPATH/bin
make install
```

### Running tests without cluster access

All advisor checks are tested using hand-built `AnalysisContext` values — no live Kubernetes, Prometheus, or AWS connections are required. Fake implementations of the K8s, Prometheus, and cloud provider interfaces are provided in each package's `_test.go` files.

```bash
go test ./internal/advisor/checks/... -v
```

---

## Roadmap

- [ ] GKE support (Cloud Billing API for pricing)
- [ ] AKS support (Azure Pricing API)
- [ ] HPA recommendation check (workloads that could autoscale instead of being over-provisioned)
- [ ] PodDisruptionBudget audit
- [x] GitHub Actions CI
- [ ] Helm chart for in-cluster deployment with scheduled reports

---

## Built on

Vibes and AI tokens. No AWS bill was harmed in the making of this tool — but yours might be smaller after running it.

---

## License

MIT
