package main

import (
	"fmt"
	"math/rand"
	"time"
)

/*
This code simulates:
- A central scheduler (`CentralUnit`) that receives HPC-like job requests.
- Three simulated clusters with CPU capacity and energy efficiency differences.
- A simple scheduling algorithm that picks the best cluster based on estimated energy cost versus priority.
*/

// Workload represents a simplified job
type Workload struct {
	ID             string
	CPURequirement int
	EnergyPriority float64 // 0.0 to 1.0, higher = more energy aware
}

// Cluster interface that each "cluster" should implement
type Cluster interface {
	Name() string
	CanAccept(w Workload) bool
	EstimateEnergyCost(w Workload) float64
	SubmitJob(w Workload) error
}

// SimulatedCluster is a stubbed implementation of Cluster
type SimulatedCluster struct {
	ClusterName string
	MaxCPU      int
	EnergyBias  float64 // Multiplier for energy estimation
}

func (c SimulatedCluster) Name() string {
	return c.ClusterName
}

func (c SimulatedCluster) CanAccept(w Workload) bool {
	return w.CPURequirement <= c.MaxCPU
}

func (c SimulatedCluster) EstimateEnergyCost(w Workload) float64 {
	return float64(w.CPURequirement) * c.EnergyBias
}

func (c SimulatedCluster) SubmitJob(w Workload) error {
	fmt.Printf("[Cluster %s] Job %s submitted (CPU: %d, EnergyBias: %.2f)\n",
		c.ClusterName, w.ID, w.CPURequirement, c.EnergyBias)
	return nil
}

// CentralUnit is the orchestrator
type CentralUnit struct {
	Clusters []Cluster
}

// Schedule selects the best cluster based on energy and submits the job
func (cu CentralUnit) Schedule(w Workload) error {
	var selected Cluster
	minCost := 1e9

	for _, cluster := range cu.Clusters {
		if !cluster.CanAccept(w) {
			continue
		}
		cost := cluster.EstimateEnergyCost(w) * w.EnergyPriority
		if cost < minCost {
			minCost = cost
			selected = cluster
		}
	}

	if selected == nil {
		return fmt.Errorf("no suitable cluster found for workload %s", w.ID)
	}

	return selected.SubmitJob(w)
}

// SimulateWorkloads generates and schedules some workloads
func main() {
	rand.Seed(time.Now().UnixNano())

	clusterA := SimulatedCluster{"eu-central", 16, 1.0}
	clusterB := SimulatedCluster{"us-west", 32, 0.8}
	clusterC := SimulatedCluster{"low-power-node", 8, 0.5}

	cu := CentralUnit{
		Clusters: []Cluster{clusterA, clusterB, clusterC},
	}

	workloads := []Workload{
		{"job-1", 10, 1.0},
		{"job-2", 20, 0.5},
		{"job-3", 6, 0.9},
		{"job-4", 32, 0.4},
		{"job-5", 4, 1.0},
	}

	for _, w := range workloads {
		err := cu.Schedule(w)
		if err != nil {
			fmt.Printf("[CentralUnit] Failed to schedule %s: %v\n", w.ID, err)
		}
	}
}
