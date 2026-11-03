# pvbench

A `kubectl` plugin for inspecting Kubernetes PersistentVolumeClaims (PVCs) and benchmarking Kubernetes storage performance with [FIO](https://fio.readthedocs.io/).

## Installation

Coming soon.

## Usage

```bash
kubectl pvbench --help
kubectl pvbench version

# Inspect an existing PVC
kubectl pvbench inspect my-pvc -n stage

# Run a read benchmark against a StorageClass
kubectl pvbench benchmark \\
  --storage-class longhorn \\
  --size 2Gi \\
  --profile read \\
  --timeout 5m \\
  -n stage
```

## Storage benchmarks

The `benchmark` command creates a temporary PVC using the selected StorageClass, runs FIO workloads against it, prints a human-readable results table, and deletes the temporary resources when the run finishes.

### Benchmark profiles

Use `--profile` to select which workload group to run:

| Profile | Workloads | Description |
|---|---|---|
| `read` | `seq-read`, `rand-read` | Measures sequential and random read performance |
| `write` | `seq-write`, `rand-write` | Measures sequential and random write performance |
| `all` | All four workloads | Runs sequential read, sequential write, random read, and random write |

The default profile is `all`.

### Workload types

Each profile consists of the following workload types:

| Workload | Access pattern | Operation | Block size |
|---|---|---|---|
| `seq-read` | Sequential | Read | 1 MiB |
| `seq-write` | Sequential | Write | 1 MiB |
| `rand-read` | Random | Read | 4 KiB |
| `rand-write` | Random | Write | 4 KiB |

- **Sequential I/O** accesses data in order. It is useful for understanding throughput for workloads such as large file transfers and streaming.
- **Random I/O** accesses data at non-sequential offsets. It is useful for understanding small, scattered I/O patterns common in databases and other transactional workloads.
- **Read workloads** measure how quickly data can be read from the volume.
- **Write workloads** measure how quickly data can be written to the volume.

### FIO settings

The current benchmark uses these FIO settings:

| Setting | Value |
|---|---|
| Benchmark file | `/pvbench-data/test.bin` |
| Benchmark file size | 1 GiB |
| I/O engine | `libaio` |
| Direct I/O | Enabled (`--direct=1`) |
| I/O depth | 16 |
| Number of jobs | 1 |
| Measurement duration | 30 seconds per workload |
| Reporting | FIO JSON, parsed internally by pvbench |

Before running the selected workloads, pvbench writes the test file to ensure that read benchmarks have data to read. Preparation output is not included in the benchmark results.

These are controlled, single-job tests. Results are useful for comparing StorageClasses under the same conditions, but they do not represent every application's I/O pattern or guarantee production performance.

### Result metrics

The benchmark summary reports these metrics for each workload:

| Metric | Meaning |
|---|---|
| **IOPS** | Number of I/O operations completed per second |
| **Bandwidth** | Data transferred per second, displayed in MiB/s |
| **Average latency** | Average completion latency, displayed in milliseconds |

FIO's JSON output is parsed internally. The summary uses the read metrics for read workloads and the write metrics for write workloads. Failed or incomplete workloads are shown as failed instead of being silently omitted.

### Benchmark options

| Flag | Default | Description |
|---|---|---|
| `--storage-class` | Required | StorageClass used to provision the temporary PVC |
| `--size` | `1Gi` | Size of the temporary PVC; the current benchmark requires at least 1 GiB |
| `--profile` | `all` | Workload profile: `read`, `write`, or `all` |
| `--timeout` | `5m` | Maximum time to wait for the benchmark Job |
| `-n, --namespace` | Current context namespace | Namespace for the temporary PVC and Job |
| `--keep-resources` | `false` | Keep the temporary PVC and Job for troubleshooting instead of deleting them |

Example: use `--profile read` to run the sequential and random read workloads, or `--profile write` to run the sequential and random write workloads.

> **Note:** The test file is currently fixed at 1 GiB. Choose a PVC size larger than the test file where possible, leaving room for filesystem overhead.

## Cleanup

By default, pvbench deletes the temporary benchmark Job and PVC after collecting the results, including when a workload fails or times out. Use `--keep-resources` when you need to inspect the resources after a run. Resources retained with this flag must be deleted manually.


## Releases

pvbench uses [Semantic Versioning](https://semver.org/) and Conventional Commits. Every change merged into `master` is evaluated by Release Please, which opens or updates a release pull request:

- `fix:` creates a PATCH release (for example, `v0.1.0` → `v0.1.1`).
- `feat:` creates a MINOR release (for example, `v0.1.0` → `v0.2.0`).
- A commit containing `BREAKING CHANGE:` in its footer, or using the `!` marker (for example, `feat!:`), creates a MAJOR release.

When the release pull request is merged, Release Please creates the version tag. The tag triggers GoReleaser, which runs the test suite and publishes archives and checksums to the GitHub Release.

### Release artifacts

The release workflow currently builds these platforms:

| OS | Architectures |
|---|---|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |

Each archive contains the `kubectl-pvbench` executable, `LICENSE`, and `README.md`. The version is embedded in the executable and is shown by `kubectl pvbench version`.

### Repository setup

A repository administrator must create a fine-grained personal access token (or equivalent GitHub App token) with **Contents: read/write** and **Pull requests: read/write** access to this repository. Save it as the repository Actions secret `RELEASE_PLEASE_TOKEN`.

Release Please uses this token to create release pull requests and tags. Using a dedicated token is important because tags created with the default `GITHUB_TOKEN` do not trigger other GitHub Actions workflows. Once configured, merging a release PR automatically starts the tagged release workflow.

### Krew distribution

The root-level `.krew.yaml` is the Krew release manifest template. It describes the supported platforms and uses the Krew release helper to insert each release archive URL and its SHA-256 checksum.

The official Krew index is a separate repository: [kubernetes-sigs/krew-index](https://github.com/kubernetes-sigs/krew-index). Its plugin manifest belongs at `plugins/pvbench.yaml` in that repository; it should not be committed as an index directory inside this source repository. Submit the initial manifest to the official index following the [Krew plugin submission guide](https://krew.sigs.k8s.io/docs/developer-guide/release/). Subsequent releases can be integrated through the Krew release bot after the plugin is accepted.

After pvbench is indexed, users can install it with:

```bash
kubectl krew install pvbench
kubectl pvbench version
```

## License

See [LICENSE](LICENSE).
