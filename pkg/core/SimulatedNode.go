package core

import (
	"container/list"
	"math"
	"time"
)

// SimulatedNode represents a cluster node
type SimulatedNode struct {
	Name            string
	TotalCPU        float64
	TotalMemory     float64
	AvailableCPU    float64
	AvailableMemory float64
	CarbonIntensity float64    // gCO₂/kWh, static or TODO fetch
	Reservations    *list.List // holds end-times for allocations
	Metadata		map[string]string
}

// CanAccept reports whether node has enough free resources
func (n *SimulatedNode) CanAccept(w Workload) bool {
	return n.AvailableCPU >= w.CPU && n.AvailableMemory >= w.Memory
}

// Reserve consumes capacity and records reservation end-time
func (n *SimulatedNode) Reserve(w Workload, start time.Time) {
    n.AvailableCPU    -= w.CPU
    n.AvailableMemory -= w.Memory
    n.Reservations.PushBack(&Reservation{
        endTime:     start.Add(w.Duration),
        cpuReserved: w.CPU,
        memReserved: w.Memory,
    })
}

// Release frees resources for all reservations ending <= t
func (n *SimulatedNode) Release(t time.Time) {
    for e := n.Reservations.Front(); e != nil; {
        next := e.Next()
        r := e.Value.(*Reservation)
        if !r.endTime.After(t) {
            // give back exactly what was reserved
            n.AvailableCPU    = math.Min(n.AvailableCPU + r.cpuReserved,  n.TotalCPU)
            n.AvailableMemory = math.Min(n.AvailableMemory + r.memReserved, n.TotalMemory)
            n.Reservations.Remove(e)
        }
        e = next
    }
}

// NewNode constructs a core.SimulatedNode with capacity and CI
func NewNode(name string, cpu, mem, ci float64) *SimulatedNode {
	n := &SimulatedNode{
		Name:            name,
		TotalCPU:        cpu,
		TotalMemory:     mem,
		AvailableCPU:    cpu,
		AvailableMemory: mem,
		CarbonIntensity: ci,
		Reservations:    list.New(),
	}
	return n
}