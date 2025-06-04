### KubeEnergyScheduler
KubeEnergyScheduler *aims* to be a platform-agnostic plugin that seamlessly integrates *heterogenous* cloud infrastructures with **sustainability** in mind.

- **Fully-managed**: the user (developer/researcher) does not have to worry about the underlying computation and resource allocation.
- **Kubernetes-bases**: Kubernetes is the *de facto* cluster framework used at the core of many cloud infrastructures.


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
...
```