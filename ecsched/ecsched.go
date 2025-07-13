package ecsched

import (
	"container/list"
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// SchedulerType determines which scheduling algorithm to use
// MCFP: cost-based dot-product + CI, Kubernetes and Swarm as baselines
// to compare per Hu et al. 2020

type SchedulerType int

const (
	MCFP SchedulerType = iota
	Kubernetes
	Swarm
)

// Workload represents a job to schedule
type Workload struct {
	ID         string
	SubmitTime time.Time
	Duration   time.Duration
	CPU        float64
	Memory     float64
}

// SimulatedNode represents a node in the cluster
// Reservations queue holds end-times for running jobs
type SimulatedNode struct {
	Name             string
	TotalCPU         float64
	TotalMemory      float64
	AvailableCPU     float64
	AvailableMemory  float64
	CarbonIntensity  float64    // gCO₂/kWh, static or TODO fetch dynamic
	Reservations     *list.List // each element: time.Time
}

// NewNode creates a node with given capacity and CI
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

// CanAccept checks if the node has enough free resources for the workload
func (n *SimulatedNode) CanAccept(w Workload) bool {
	return n.AvailableCPU >= w.CPU && n.AvailableMemory >= w.Memory
}

// Reserve consumes resources and records the end time of the job
func (n *SimulatedNode) Reserve(w Workload, start time.Time) {
	n.AvailableCPU -= w.CPU
	n.AvailableMemory -= w.Memory
	n.Reservations.PushBack(start.Add(w.Duration))
}

// Release frees any resources whose recorded end-time is <= t
func (n *SimulatedNode) Release(t time.Time) {
	for e := n.Reservations.Front(); e != nil; {
		next := e.Next()
		end := e.Value.(time.Time)
		if !end.After(t) {
			// approximate full release; TODO: track per-job resource
			n.AvailableCPU = math.Min(n.AvailableCPU+wPlaceholderCPU, n.TotalCPU)
			n.AvailableMemory = math.Min(n.AvailableMemory+wPlaceholderMem, n.TotalMemory)
			n.Reservations.Remove(e)
		}
		e = next
	}
}

// DiscreteEventScheduler drives the event loop
type DiscreteEventScheduler struct {
	Clock     time.Time
	Nodes     []*SimulatedNode
	Events    []Event  // sorted by Time
	Logs      []string // CSV lines: job,node,submit,start,end
	SchedType SchedulerType
}

type EventType int

const (
	JobArrival EventType = iota
	JobEnd
)

type Event struct {
	Time     time.Time
	Type     EventType
	Workload Workload
	Node     *SimulatedNode
}

// NewScheduler initializes a scheduler with nodes and a strategy
func NewScheduler(nodes []*SimulatedNode, st SchedulerType) *DiscreteEventScheduler {
	return &DiscreteEventScheduler{
		Clock:     time.Now(),
		Nodes:     nodes,
		Events:    []Event{},
		Logs:      []string{},
		SchedType: st,
	}
}

// AddWorkload enqueues an arrival event
func (s *DiscreteEventScheduler) AddWorkload(w Workload) {
	s.Events = append(s.Events, Event{Time: w.SubmitTime, Type: JobArrival, Workload: w})
}

// Run processes all events in time order
func (s *DiscreteEventScheduler) Run() {
	s.sortEvents()
	for len(s.Events) > 0 {
		e := s.Events[0]
		s.Events = s.Events[1:]
		s.Clock = e.Time
		// release resources first
		s.processReleases(s.Clock)
		s.handleEvent(e)
	}
}

func (s *DiscreteEventScheduler) sortEvents() {
	sort.Slice(s.Events, func(i, j int) bool {
		return s.Events[i].Time.Before(s.Events[j].Time)
	})
}

// processReleases frees resources for ended jobs
func (s *DiscreteEventScheduler) processReleases(t time.Time) {
	for _, n := range s.Nodes {
		n.Release(t)
	}
}

// handleEvent dispatches arrival or end
func (s *DiscreteEventScheduler) handleEvent(e Event) {
	switch e.Type {
	case JobArrival:
		scheduleNode := s.selectNode(e.Workload)
		if scheduleNode != nil {
			s.reserveAndLog(scheduleNode, e.Workload)
		} else {
			log.Printf("Job %s could not be scheduled", e.Workload.ID)
		}
	case JobEnd:
		log.Printf("Job %s ended on %s at %v", e.Workload.ID, e.Node.Name, s.Clock)
	}
}

// reserveAndLog reserves resources, enqueues end event, and logs CSV
func (s *DiscreteEventScheduler) reserveAndLog(n *SimulatedNode, w Workload) {
	n.Reserve(w, s.Clock)
	s.Events = append(s.Events, Event{Time: s.Clock.Add(w.Duration), Type: JobEnd, Node: n, Workload: w})
	s.Logs = append(s.Logs, fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, n.Name, w.SubmitTime, s.Clock, s.Clock.Add(w.Duration)))
	log.Printf("Scheduled %s on %s at %v", w.ID, n.Name, s.Clock)
}

// selectNode picks a node based on the chosen strategy
func (s *DiscreteEventScheduler) selectNode(w Workload) *SimulatedNode {
	switch s.SchedType {
	case Kubernetes:
		return s.scheduleKubernetes(w)
	case Swarm:
		return s.scheduleSwarm(w)
	case MCFP:
		return s.scheduleMCFP(w)
	default:
		return nil
	}
}

// scheduleKubernetes selects least-loaded node (sum usage minimal)
func (s *DiscreteEventScheduler) scheduleKubernetes(w Workload) *SimulatedNode {
	cands := []*SimulatedNode{}
	for _, n := range s.Nodes {
		if n.CanAccept(w) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool {
		used1 := (cands[i].TotalCPU-cands[i].AvailableCPU)/cands[i].TotalCPU + (cands[i].TotalMemory-cands[i].AvailableMemory)/cands[i].TotalMemory
		used2 := (cands[j].TotalCPU-cands[j].AvailableCPU)/cands[j].TotalCPU + (cands[j].TotalMemory-cands[j].AvailableMemory)/cands[j].TotalMemory
		if used1 != used2 {
			return used1 < used2
		}
		return cands[i].Name < cands[j].Name
	})
	return cands[0]
}

// scheduleSwarm selects most-loaded node (sum usage maximal)
func (s *DiscreteEventScheduler) scheduleSwarm(w Workload) *SimulatedNode {
	cands := []*SimulatedNode{}
	for _, n := range s.Nodes {
		if n.CanAccept(w) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool {
		used1 := (cands[i].TotalCPU-cands[i].AvailableCPU)/cands[i].TotalCPU + (cands[i].TotalMemory-cands[i].AvailableMemory)/cands[i].TotalMemory
		used2 := (cands[j].TotalCPU-cands[j].AvailableCPU)/cands[j].TotalCPU + (cands[j].TotalMemory-cands[j].AvailableMemory)/cands[j].TotalMemory
		if used1 != used2 {
			return used1 > used2
		}
		return cands[i].Name < cands[j].Name
	})
	return cands[0]
}

// scheduleMCFP uses dot-product + CI cost model to pick min-cost node
func (s *DiscreteEventScheduler) scheduleMCFP(w Workload) *SimulatedNode {
	var best *SimulatedNode
	bestCost := math.MaxFloat64
	for _, n := range s.Nodes {
		if !n.CanAccept(w) {
			continue
		}
		rawDP := w.CPU*n.TotalCPU + w.Memory*n.TotalMemory
		rawCI := n.CarbonIntensity
		// cost = -DP + 0.1*CI (higher DP lower cost, CI penalized lightly)
		cost := -rawDP + 0.1*rawCI
		log.Printf("MCFP cost for Job %s->Node %s: DP=%.2f, CI=%.2f, cost=%.2f", w.ID, n.Name, rawDP, rawCI, cost)
		if cost < bestCost || best == nil {
			bestCost = cost
			best = n
		}
	}
	return best
}

```go
// scheduleBatch is a helper to batch up workloads, build the flow network,
// solve MCFP once, and return a map of assignments.
func (s *DiscreteEventScheduler) scheduleBatch(ws []Workload) map[string]*SimulatedNode {
	assign := make(map[string]*SimulatedNode)
	// TODO: 1) build flow graph over ws and s.Nodes
	//       2) run min-cost flow solver
	//       3) extract mappings container->node into assign
	// placeholder: fall back to per-workload MCFP
	for _, w := range ws {
		assign[w.ID] = s.scheduleMCFP(w)
	}
	return assign
}


// scheduleBatch batches pending workloads into an MCFP and assigns as many as possible
func (s *Scheduler) scheduleBatch() {
	n := len(s.pending)
	if n == 0 {
		return
	}
	m := len(s.Nodes)

	// 1) Log batch size
	log.Printf("→ scheduleBatch: batching %d pending jobs", n)

	// Graph offsets
	src := 0
	workOff := 1
	nodeOff := workOff + n
	unsched := nodeOff + m
	sink := unsched + 1
	N := sink + 1

	g := newGraph(N)

	// src -> workloads
	for i := 0; i < n; i++ {
		g.addEdge(src, workOff+i, 1, 0)
	}

	// workloads -> machines & unscheduled
	for i, w := range s.pending {
		for j, node := range s.Nodes {
			if node.CanAccept(w) {
				// compute dot-product
				rawDP := w.CPU*node.TotalCPU + w.Memory*node.TotalMemory
				// compute CI score: TODO refine formula or fetch dynamic metrics
				rawCI := node.CarbonIntensity

				// combine costs: weight dp and ci
				costF := -rawDP + 0.1*rawCI // dp prioritized, CI penalizes
				cost := int(costF * 1000)

				log.Printf("   • Job %s->Node %s: DP=%.2f, CI=%.2f, cost=%d", w.ID, node.Name, rawDP, rawCI, cost)
				g.addEdge(workOff+i, nodeOff+j, 1, cost)
			}
		}
		// fallback unscheduled
		g.addEdge(workOff+i, unsched, 1, 0)
	}

	// machines -> sink
	for j := 0; j < m; j++ {
		g.addEdge(nodeOff+j, sink, 1, 0)
	}
	// unscheduled -> sink
	g.addEdge(unsched, sink, n, 0)

	// 2) Run MCFP
	flow, _ := g.minCostMaxFlow(src, sink)
	log.Printf("← scheduleBatch: MCFP assigned %d/%d jobs", flow, n)
	if flow == 0 {
		return
	}

	// extract assignments
	newPending := make([]Workload, 0, n)
	for i, w := range s.pending {
		assigned := false
		for _, e := range g.adj[workOff+i] {
			if e.to >= nodeOff && e.to < nodeOff+m && e.flow > 0 {
				j := e.to - nodeOff
				node := s.Nodes[j]

				log.Printf("   → Assign Job %s to Node %s (flow=%d)", w.ID, node.Name, e.flow)

				node.Reserve(w, s.Clock)
				s.timeline = append(s.timeline, Event{
					Time:     s.Clock.Add(w.Duration),
					Type:     End,
					Workload: w,
					Node:     node,
				})
				s.Logs = append(s.Logs,
					fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, node.Name, w.SubmitTime, s.Clock, s.Clock.Add(w.Duration)))
				assigned = true
				break
			}
		}
		if !assigned {
			newPending = append(newPending, w)
		}
	}
	s.pending = newPending
}
```

// TODO placeholders until job-specific CPU/Memory tracked during release
const (
	wPlaceholderCPU = 0.0
	wPlaceholderMem = 0.0
)
