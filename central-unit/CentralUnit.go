package main

import (
	"fmt"
	"math/rand"
	"time"
	"reflect"
)

/*
This code simulates:
- A central scheduler (`CentralUnit`) that receives HPC-like job requests.
- Three simulated clusters with CPU capacity and energy efficiency differences.
- A simple scheduling algorithm that picks the best cluster based on estimated energy cost versus priority.
*/

// --- Types ---

type Workload struct {
	ID             string
	CPURequirement int
	EnergyPriority float64 // 0.0 to 1.0, higher = more energy aware
}

type Cluster interface {
	Name() string
	CanAccept(w Workload) bool
	EstimateEnergyCost(w Workload) float64
	SubmitJob(w Workload) error
	CarbonIntensity() float64 // For logging
}

type SimulatedCluster struct {
	ClusterName     string
	MaxCPU          int
	EnergyBias      float64
	SCI_kWh         float64
	CurrentLoad     float64
	Location        string
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
	fmt.Printf("[Cluster %s] Job %s submitted (CPU: %d, EnergyBias: %.2f, SCI: %.1f)\n",
		c.ClusterName, w.ID, w.CPURequirement, c.EnergyBias, c.SCI_kWh)
	return nil
}
func (c SimulatedCluster) CarbonIntensity() float64 {
	return c.SCI_kWh
}

type SchedulingStrategy interface {
	SelectCluster([]Cluster, Workload) (Cluster, string, error) // string = reasoning
}

type EnergyAwareStrategy struct{}

// Picks cluster with lowest energy cost that can accept workload
func (s EnergyAwareStrategy) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	var best Cluster
	var minCost float64 = 1e9
	for _, c := range clusters {
		if !c.CanAccept(w) {
			continue
		}
		cost := c.EstimateEnergyCost(w)
		if cost < minCost {
			minCost = cost
			best = c
		}
	}
	if best == nil {
		return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
	}
	reason := fmt.Sprintf("Selected %s with lowest energy cost: %.2f", best.Name(), minCost)
	return best, reason, nil
}

type CentralUnit struct {
	Clusters  []Cluster
	Strategy  SchedulingStrategy
}

// -- Scheduling Decision Logging --

type SchedulingDecision struct {
	WorkloadID      string
	StrategyName    string
	SelectedCluster string
	EstimatedCost   float64
	SCI_kWh         float64
	Timestamp       time.Time
	Reasoning       string
}

var decisionLog []SchedulingDecision

// func GetCarbonIntensity(region string) float64 {
//     // This is just a placeholder; you'd use the actual API and parse JSON.
//     resp, err := http.Get("https://api.electricitymaps.com/v3/carbon-intensity?zone=" + region)
//     if err != nil { return -1 }
//     // Parse JSON (use encoding/json)
//     // ...
//     return value
// }


func main() {
	rand.Seed(time.Now().UnixNano())

	clusterA := SimulatedCluster{"eu-central", 16, 1.0, 500, 0, "EU"}
	clusterB := SimulatedCluster{"us-west", 32, 0.8, 350, 0, "US"}
	clusterC := SimulatedCluster{"low-power-node", 8, 0.5, 50, 0, "NL"}

	cu := CentralUnit{
		Clusters: []Cluster{clusterA, clusterB, clusterC},
		Strategy: EnergyAwareStrategy{},
	}

	workloads := []Workload{
		{"job-1", 10, 1.0},
		{"job-2", 20, 0.5},
		{"job-3", 6, 0.9},
		{"job-4", 32, 0.4},
		{"job-5", 4, 1.0},
	}

	for _, w := range workloads {
		selected, reason, err := cu.Strategy.SelectCluster(cu.Clusters, w)
		if err != nil {
			fmt.Printf("[CentralUnit] Failed to schedule %s: %v\n", w.ID, err)
			continue
		}
		selected.SubmitJob(w)
		decision := SchedulingDecision{
			WorkloadID:      w.ID,
			StrategyName:    reflect.TypeOf(cu.Strategy).Name(),
			SelectedCluster: selected.Name(),
			EstimatedCost:   selected.EstimateEnergyCost(w),
			SCI_kWh:         selected.CarbonIntensity(),
			Timestamp:       time.Now(),
			Reasoning:       reason,
		}
		decisionLog = append(decisionLog, decision)
	}

	fmt.Println("\nDecision log:")
	for _, d := range decisionLog {
		fmt.Printf("%+v\n", d)
	}
}
