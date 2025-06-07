package core

import (
	"fmt"
)

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