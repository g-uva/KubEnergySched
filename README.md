### KubeEnergyScheduler
KubeEnergyScheduler *aims* to be a platform-agnostic plugin that seamlessly integrates *heterogenous* cloud infrastructures with **sustainability** in mind.

- **Fully-managed**: the user (developer/researcher) does not have to worry about the underlying computation and resource allocation.
- **Kubernetes-based**: Kubernetes is the *de facto* cluster framework used at the core of many cloud infrastructures.

### Testbed Architecture (WIP)
![Testbed Architecture](assets/testbed_architecture.png)

### Folder structure view
```txt
kube-energy-scheduler/
├── main.go
├── scheduler/
│   ├── cluster.go
│   ├── strategy.go
│   └── workload.go
├── benchmark/
│   ├── adapter.go
│   └── generator.go
├── metrics/
│   └── prometheus.go
├── data/
│   └── workloads.csv
├── go.mod
├── go.sum
...
```

- `scheduler/cluster.go`: Defines the Cluster interface and `SimulatedCluster` struct.
- `scheduler/strategy.go`: Implements various scheduling strategies (FCFS, RoundRobin, MinMin, MaxMin, EnergyAware).
- `scheduler/workload.go`: Defines the Workload struct and functions to load workloads from CSV.
- `benchmark/adapter.go`: Contains the `BenchmarkAdapter` struct to run benchmarks and export results.
- `benchmark/generator.go`: Includes functions to generate synthetic workloads based on real-world patterns.
- `metrics/prometheus.go`: Sets up Prometheus metrics and exposes them via an HTTP server.
`main.go`: Entry point that ties everything together.