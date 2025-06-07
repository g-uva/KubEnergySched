package main

import (
	"fmt"
	"reflect"
	"time"
	"encoding/json"
	"os"
	"bufio"
	"net/http"
	"strconv"
	"strings"
	"bytes"
)

type RemoteCluster struct {
	NameKey string `json:"name"`
	URL  string `json:"url"`
}

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
	ClusterName string
	MaxCPU      int
	EnergyBias  float64
	SCI_kWh     float64
	CurrentLoad float64
	Location    string
}

func (c SimulatedCluster) Name() string              { return c.ClusterName }
func (c SimulatedCluster) CanAccept(w Workload) bool { return w.CPURequirement <= c.MaxCPU }
func (c SimulatedCluster) EstimateEnergyCost(w Workload) float64 {
	return float64(w.CPURequirement) * c.EnergyBias
}
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
	n := len(clusters)
	for i := 0; i < n; i++ {
		c := clusters[i]
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
	for i := 0; i < n; i++ {
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
	n := len(clusters)
	for i := 0; i < n; i++ {
		c := clusters[i]
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
	n := len(clusters)
	for i := 0; i < n; i++ {
		c := clusters[i]
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
	n := len(clusters)
	for i := 0; i < n; i++ {
		c := clusters[i]
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

func (c RemoteCluster) Name() string { return c.NameKey }

func (c RemoteCluster) CanAccept(w Workload) bool {
	cpu, err := c.GetMetricValue("compute_node_cpu_usage")
	if err != nil {
		return false
	}
	// Assume node can accept job if CPU is below a threshold
	return cpu < 90.0
}

func (c RemoteCluster) EstimateEnergyCost(w Workload) float64 {
	// Simplified: base cost = CPU × 1.0
	return float64(w.CPURequirement)
}

func (c RemoteCluster) SubmitJob(w Workload) error {
	payload := map[string]interface{}{
		"id": w.ID,
		"cpu": w.CPURequirement,
		"priority": w.EnergyPriority,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.URL+"/submit", "application/json", bytes.NewReader(jsonData))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("submit failed: status %d", resp.StatusCode)
	}

	fmt.Printf("[RemoteCluster %s] Job %s submitted (CPU: %d, Priority: %.2f)\n",
		c.Name, w.ID, w.CPURequirement, w.EnergyPriority)
	return nil
}

func (c RemoteCluster) CarbonIntensity() float64 {
	// Placeholder — could scrape a real metric
	return 300.0
}

func (c RemoteCluster) GetMetricValue(metricName string) (float64, error) {
	resp, err := http.Get(c.URL + "/metrics")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, metricName) {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return strconv.ParseFloat(parts[1], 64)
			}
		}
	}
	return 0, fmt.Errorf("metric %s not found", metricName)
}

func main() {
	fmt.Println("Loading clusters from file...")
	clustersRaw, err := LoadClustersFromFile("/config/clusters.json")
	if err != nil {
		fmt.Println("Error loading clusters:", err)
		return
	}
	fmt.Printf("Loaded %d clusters from file\n", len(clustersRaw))

	// Wrap into []Cluster interface
	var clusters []Cluster
	for _, rc := range clustersRaw {
		clusters = append(clusters, rc)
	}

	unit := CentralUnit{
		Clusters: clusters,
		Strategy: &RoundRobin{},
	}

	workloads := []Workload{
		{ID: "job1", CPURequirement: 4, EnergyPriority: 0.7},
		{ID: "job2", CPURequirement: 2, EnergyPriority: 0.4},
	}

	unit.Dispatch(workloads)
}