package main

import (
    "kube-scheduler/central-unit"
    "kube-scheduler/benchmark"
)

func main() {
    clusters := []centralunit.Cluster{
        centralunit.SimulatedCluster{"eu-central", 16, 1.0, 500, 0, "EU"},
        centralunit.SimulatedCluster{"us-west", 32, 0.8, 350, 0, "US"},
        centralunit.SimulatedCluster{"low-power-node", 8, 0.5, 50, 0, "NL"},
    }

    workloads := []centralunit.Workload{
        {"job-1", 10, 1.0},
        {"job-2", 20, 0.5},
        {"job-3", 6, 0.9},
        {"job-4", 32, 0.4},
        {"job-5", 4, 1.0},
    }

    strategies := []centralunit.SchedulingStrategy{
        centralunit.FCFS{},
        &centralunit.RoundRobin{},
        centralunit.MinMin{},
        centralunit.MaxMin{},
        centralunit.EnergyAwareStrategy{},
    }

    adapter := benchmark.BenchmarkAdapter{
        Clusters:   clusters,
        Strategies: strategies,
        Workloads:  workloads,
    }

    adapter.RunBenchmark()
    adapter.ExportToCSV("benchmark_results.csv")
}
