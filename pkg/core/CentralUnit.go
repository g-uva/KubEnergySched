package core

import (
	"fmt"
	"reflect"
	"time"
	"os"
	"encoding/json"
)

type CentralUnit struct {
	Clusters []Cluster
	Strategy SchedulingStrategy
}

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

func (cu CentralUnit) Dispatch(workloads []Workload) {
	n := len(workloads)
	for i := 0; i < n; i++ {
		w := workloads[i]
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
}

func LoadClustersFromFile(path string) ([]RemoteCluster, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var clusters []RemoteCluster
	err = json.NewDecoder(file).Decode(&clusters)
	return clusters, err
}