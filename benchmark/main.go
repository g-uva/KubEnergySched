package main

import (
	"kube-scheduler/pkg/core"
	benchmark "kube-scheduler/benchmark/components"
)

func main() {
	clusters, _ := core.LoadClustersFromFile("config/clusters.json")

	ba := benchmark.BenchmarkAdapter{
		Clusters:   toClusterInterface(clusters),
		Strategies: []core.SchedulingStrategy{
			&core.RoundRobin{},
			&core.MinMin{},
			&core.MaxMin{},
			&core.EnergyAwareStrategy{},
		},
		Workloads: benchmark.GenerateSyntheticWorkloads(50),
	}

	ba.RunBenchmark()
	ba.ExportToCSV()
}

func toClusterInterface(rc []core.RemoteCluster) []core.Cluster {
	var result []core.Cluster
	for _, c := range rc {
		result = append(result, c)
	}
	return result
}