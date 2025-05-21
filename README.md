### kube-energy-scheduler

For the moment this is a standalone scheduler binary that registers two toy plugins:
– EnergyEfficiencyPlugin (linear power-efficiency model)
– DVFSPlugin (favors mid-range CPU frequencies)

### File view
```txt
energy-scheduler/
├── go.mod
├── go.sum
├── main.go
├── plugins/
│   ├── energy.go
│   └── dvfs.go
└── Dockerfile
```

### Testbed view
```txt
┌────────────────────────────┐
│    User Workload Request   │
└────────────┬───────────────┘
             ▼
   ┌────────────────────┐
   │  Central Unit (Go) │ ← Modular scheduler logic
   └────┬────┬────┬─────┘
        │    │    │
        ▼    ▼    ▼
   ┌──────┐ ┌──────┐ ┌──────┐
   │ K8sA │ │ K8sB │ │ K8sC │  ← Simulated heterogeneous clusters (labels, plugins, etc.)
   └──────┘ └──────┘ └──────┘

          ▲
          │ (Metrics)
     Prometheus + Grafana
```