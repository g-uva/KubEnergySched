package centralunit

import (
	"fmt"
	"math/rand"
	"reflect"
	"time"
)

/*
Next I need the code o be exposed at /metrics in Promtheus. Possibly I need to create a Go server and exporter with a Prometheus client library.
I already have a configuration for each POD with a Scaphandre agent sidecar.
*/

// --- Types (As in your original) ---
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
	CarbonIntensity() float64
}

type SimulatedCluster struct {
	ClusterName     string
	MaxCPU          int
	EnergyBias      float64
	SCI_kWh         float64
	CurrentLoad     float64
	Location        string
}

func (c SimulatedCluster) Name() string                        { return c.ClusterName }
func (c SimulatedCluster) CanAccept(w Workload) bool          { return w.CPURequirement <= c.MaxCPU }
func (c SimulatedCluster) EstimateEnergyCost(w Workload) float64 { return float64(w.CPURequirement) * c.EnergyBias }
func (c SimulatedCluster) SubmitJob(w Workload) error {
	fmt.Printf("[Cluster %s] Job %s submitted (CPU: %d, EnergyBias: %.2f, SCI: %.1f)\n",
		c.ClusterName, w.ID, w.CPURequirement, c.EnergyBias, c.SCI_kWh)
	return nil
}
func (c SimulatedCluster) CarbonIntensity() float64 { return c.SCI_kWh }

// --- Scheduling Strategies ---
type SchedulingStrategy interface {
	SelectCluster([]Cluster, Workload) (Cluster, string, error)
}

type FCFS struct{}
func (s FCFS) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	for _, c := range clusters {
		if c.CanAccept(w) {
			return c, "Selected first available cluster", nil
		}
	}
	return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
}

type RoundRobin struct {
	counter int
}
func (s *RoundRobin) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	n := len(clusters)
	for i := range n {
		c := clusters[(s.counter+i)%n]
		if c.CanAccept(w) {
			s.counter = (s.counter + 1) % n
			return c, fmt.Sprintf("Selected %s using Round Robin", c.Name()), nil
		}
	}
	return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
}

type MinMin struct{}
func (s MinMin) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	var best Cluster
	minCost := 1e9
	for _, c := range clusters {
		if c.CanAccept(w) {
			cost := c.EstimateEnergyCost(w)
			if cost < minCost {
				minCost = cost
				best = c
			}
		}
	}
	if best == nil {
		return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
	}
	return best, fmt.Sprintf("Min-Min selected %s with cost %.2f", best.Name(), minCost), nil
}

type MaxMin struct{}
func (s MaxMin) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	var best Cluster
	maxCost := -1.0
	for _, c := range clusters {
		if c.CanAccept(w) {
			cost := c.EstimateEnergyCost(w)
			if cost > maxCost {
				maxCost = cost
				best = c
			}
		}
	}
	if best == nil {
		return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
	}
	return best, fmt.Sprintf("Max-Min selected %s with cost %.2f", best.Name(), maxCost), nil
}

type EnergyAwareStrategy struct{}
func (s EnergyAwareStrategy) SelectCluster(clusters []Cluster, w Workload) (Cluster, string, error) {
	var best Cluster
	minCost := 1e9
	for _, c := range clusters {
		if !c.CanAccept(w) { continue }
		cost := c.EstimateEnergyCost(w)
		if cost < minCost {
			minCost = cost
			best = c
		}
	}
	if best == nil {
		return nil, "No cluster can accept job", fmt.Errorf("no cluster can accept job")
	}
	return best, fmt.Sprintf("Selected %s with lowest energy cost: %.2f", best.Name(), minCost), nil
}

// --- Central Unit ---
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
}

func main() {
	rand.Seed(time.Now().UnixNano())

	clusters := []Cluster{
		SimulatedCluster{"eu-central", 16, 1.0, 500, 0, "EU"},
		SimulatedCluster{"us-west", 32, 0.8, 350, 0, "US"},
		SimulatedCluster{"low-power-node", 8, 0.5, 50, 0, "NL"},
	}
	workloads := []Workload{
		{"job-1", 10, 1.0},
		{"job-2", 20, 0.5},
		{"job-3", 6, 0.9},
		{"job-4", 32, 0.4},
		{"job-5", 4, 1.0},
	}

	// Switch between strategies
	strategies := []SchedulingStrategy{
		FCFS{}, &RoundRobin{}, MinMin{}, MaxMin{}, EnergyAwareStrategy{},
	}
	for _, strategy := range strategies {
		fmt.Printf("\n--- Running with strategy: %s ---\n", reflect.TypeOf(strategy).Name())
		cu := CentralUnit{Clusters: clusters, Strategy: strategy}
		cu.Dispatch(workloads)
	}

	// Export decision log
	fmt.Println("\n--- Decision Log ---")
	for _, d := range decisionLog {
		fmt.Printf("%+v\n", d)
	}
}
