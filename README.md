### kube-energy-scheduler

For the moment this is a standalone scheduler binary that registers two toy plugins:
– EnergyEfficiencyPlugin (linear power-efficiency model)
– DVFSPlugin (favors mid-range CPU frequencies)

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