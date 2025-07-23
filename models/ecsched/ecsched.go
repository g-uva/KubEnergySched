package ecsched

import (
	"log"
	"math"
	"sort"
	"time"
	metrics "kube-scheduler/pkg/metrics"
	"kube-scheduler/pkg/core"
)

// SchedulerType selects algorithm for scheduling
// MCFP: cost-based dot-product + CI
// Kubernetes: least-loaded
// Swarm: most-loaded

type SchedulerType int

const (
	MCFP SchedulerType = iota
	Kubernetes
	Swarm
)

// LogEntry captures one placement decision
type LogEntry struct {
	JobID  string
	Node   string
	Submit time.Time
	Start  time.Time
	End    time.Time
	WaitMS int64
	CICost float64
}

// DiscreteEventScheduler drives simulation over events
type DiscreteEventScheduler struct {
	Clock     time.Time
	Nodes     []*core.SimulatedNode
	Events    []Event
	Logs      []LogEntry
	SchedType SchedulerType
	CIBaseWeight float64
	CIDynAlpha float64
	Pending 	[]core.Workload
	ScheduleBatchSize int
}

type EventType int

const (
	JobArrival EventType = iota
	JobEnd
)
type Event struct {
	Time     time.Time
	Type     EventType
	Workload core.Workload
	Node     *core.SimulatedNode
}

// NewScheduler initializes with nodes and defaults to MCFP
func NewScheduler(nodes []*core.SimulatedNode) *DiscreteEventScheduler {
	return &DiscreteEventScheduler{
		Clock:     time.Now(),
		Nodes:     nodes,
		Events:    []Event{},
		Logs:      []LogEntry{},
		SchedType: MCFP,
		CIBaseWeight: 0.1,
        	CIDynAlpha:   1.0,
	}
}

// AddWorkload enqueues arrival
func (s *DiscreteEventScheduler) AddWorkload(w core.Workload) {
	s.Events = append(s.Events, Event{Time: w.SubmitTime, Type: JobArrival, Workload: w})
}

// Run executes events in time order
func (s *DiscreteEventScheduler) Run() {
	s.sortEvents()
	for len(s.Events) > 0 {
		e := s.Events[0]
		s.Events = s.Events[1:]
		s.Clock = e.Time
		s.processReleases(s.Clock)
		s.handleEvent(e)
	}
}

func (s *DiscreteEventScheduler) sortEvents() {
	sort.Slice(s.Events, func(i, j int) bool {
		return s.Events[i].Time.Before(s.Events[j].Time)
	})
}

func (s *DiscreteEventScheduler) processReleases(t time.Time) {
    // 1) Free up any reservations whose endTime ≤ t
    for _, n := range s.Nodes {
        n.Release(t)
    }

    // 2) Try to schedule any pending workloads now that resources freed up
    var stillPending []core.Workload
    for _, w := range s.Pending {
        if n := s.selectNode(w); n != nil {
            // Reserve resources at time t
            n.Reserve(w, t)

            // Enqueue the JobEnd event for this backfilled job
            s.Events = append(s.Events, Event{
                Time:     t.Add(w.Duration),
                Type:     JobEnd,
                Node:     n,
                Workload: w,
            })

            // Log with non-zero wait
		//   ciCost := n.CarbonIntensity * w.Duration.Seconds()
		  ciCost := metrics.ComputeCICost(n, w, s.Clock)
            s.Logs = append(s.Logs, LogEntry{
                JobID:  w.ID,
                Node:   n.Name,
                Submit: w.SubmitTime,
                Start:  t,
                End:    t.Add(w.Duration),
                WaitMS: int64(t.Sub(w.SubmitTime) / time.Millisecond),
			 CICost: ciCost,
            })
        } else {
            // Still can't fit, keep in pending queue
            stillPending = append(stillPending, w)
        }
    }
    s.Pending = stillPending
}


func (s *DiscreteEventScheduler) handleEvent(e Event) {
	switch e.Type {
	case JobArrival:
		node := s.selectNode(e.Workload)
		if node != nil {
			s.reserveAndLog(node, e.Workload)
		} else {
			s.Pending = append(s.Pending, e.Workload)
			log.Printf("Job %s could not be scheduled", e.Workload.ID)
		}
	case JobEnd:
		log.Printf("Job %s ended on %s at %v", e.Workload.ID, e.Node.Name, s.Clock)
	}
}

func (s *DiscreteEventScheduler) reserveAndLog(n *core.SimulatedNode, w core.Workload) {
	n.Reserve(w, s.Clock)
	s.Events = append(s.Events, Event{Time: s.Clock.Add(w.Duration), Type: JobEnd, Node: n, Workload: w})
	// ciCost := n.CarbonIntensity * w.Duration.Seconds()
	ciCost := metrics.ComputeCICost(n, w, s.Clock)
	s.Logs = append(s.Logs, LogEntry{
		JobID: w.ID,
		Node: n.Name,
		Submit: w.SubmitTime,
		Start: s.Clock,
		End: s.Clock.Add(w.Duration),
		WaitMS: int64(s.Clock.Sub(w.SubmitTime) / time.Millisecond),
		CICost: ciCost,
	})
	log.Printf("Scheduled %s on %s at %v", w.ID, n.Name, s.Clock)
}

func (s *DiscreteEventScheduler) selectNode(w core.Workload) *core.SimulatedNode {
	switch s.SchedType {
	case Kubernetes:
		return s.scheduleKubernetes(w)
	case Swarm:
		return s.scheduleSwarm(w)
	case MCFP:
		return s.scheduleMCFP(w)
	}
	return nil
}

func (s *DiscreteEventScheduler) scheduleKubernetes(w core.Workload) *core.SimulatedNode {
	cands := []*core.SimulatedNode{}
	for _, n := range s.Nodes {
		if n.CanAccept(w) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool {
		u1 := (cands[i].TotalCPU-cands[i].AvailableCPU)/cands[i].TotalCPU + (cands[i].TotalMemory-cands[i].AvailableMemory)/cands[i].TotalMemory
		u2 := (cands[j].TotalCPU-cands[j].AvailableCPU)/cands[j].TotalCPU + (cands[j].TotalMemory-cands[j].AvailableMemory)/cands[j].TotalMemory
		if u1 != u2 {
			return u1 < u2
		}
		return cands[i].Name < cands[j].Name
	})
	return cands[0]
}

func (s *DiscreteEventScheduler) scheduleSwarm(w core.Workload) *core.SimulatedNode {
	cands := []*core.SimulatedNode{}
	for _, n := range s.Nodes {
		if n.CanAccept(w) {
			cands = append(cands, n)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool {
		u1 := (cands[i].TotalCPU-cands[i].AvailableCPU)/cands[i].TotalCPU + (cands[i].TotalMemory-cands[i].AvailableMemory)/cands[i].TotalMemory
		u2 := (cands[j].TotalCPU-cands[j].AvailableCPU)/cands[j].TotalCPU + (cands[j].TotalMemory-cands[j].AvailableMemory)/cands[j].TotalMemory
		if u1 != u2 {
			return u1 > u2
		}
		return cands[i].Name < cands[j].Name
	})
	return cands[0]
}

func (s *DiscreteEventScheduler) scheduleMCFP(w core.Workload) *core.SimulatedNode {
	// 1) compute CI volatility
	var sum, sumSq float64
	for _, n := range s.Nodes {
		ci := n.CarbonIntensity
		sum += ci
		sumSq += ci * ci
	}
	mean := sum / float64(len(s.Nodes))
	variance := sumSq/float64(len(s.Nodes)) - mean*mean
	stddev := math.Sqrt(variance)

	// 2) inflate the base weight by (1 + alpha * CV)
	dynWeight := s.CIBaseWeight * (1 + s.CIDynAlpha*(stddev/mean))

	// 3) pick the node with minimal cost = −DP + dynWeight*CI
	var best *core.SimulatedNode
	bestCost := math.MaxFloat64
	for _, n := range s.Nodes {
		if !n.CanAccept(w) {
			continue
		}
		rawDP := w.CPU*n.TotalCPU + w.Memory*n.TotalMemory
		rawCI := n.CarbonIntensity
		cost := -rawDP+dynWeight*rawCI
		log.Printf("MCFP cost for Job %s->Node %s: DP=%.2f, CI=%.2f, cost=%.2f", w.ID, n.Name, rawDP, rawCI, cost)
		if cost < bestCost {
			bestCost = cost
			best = n
		}
	}
	return best
}


// // scheduleBatch is a helper to batch up workloads, build the flow network,
// // solve MCFP once, and return a map of assignments.
// func (s *DiscreteEventScheduler) scheduleBatch(ws []core.Workload) map[string]*core.SimulatedNode {
// 	assign := make(map[string]*core.SimulatedNode)
// 	// TODO: 1) build flow graph over ws and s.Nodes
// 	//       2) run min-cost flow solver
// 	//       3) extract mappings container->node into assign
// 	// placeholder: fall back to per-workload MCFP
// 	for _, w := range ws {
// 		assign[w.ID] = s.scheduleMCFP(w)
// 	}
// 	return assign
// }


// // scheduleBatch batches pending workloads into an MCFP and assigns as many as possible
// func (s *Scheduler) scheduleBatch() {
// 	n := len(s.pending)
// 	if n == 0 {
// 		return
// 	}
// 	m := len(s.Nodes)

// 	// 1) Log batch size
// 	log.Printf("→ scheduleBatch: batching %d pending jobs", n)

// 	// Graph offsets
// 	src := 0
// 	workOff := 1
// 	nodeOff := workOff + n
// 	unsched := nodeOff + m
// 	sink := unsched + 1
// 	N := sink + 1

// 	g := newGraph(N)

// 	// src -> workloads
// 	for i := 0; i < n; i++ {
// 		g.addEdge(src, workOff+i, 1, 0)
// 	}

// 	// workloads -> machines & unscheduled
// 	for i, w := range s.pending {
// 		for j, node := range s.Nodes {
// 			if node.CanAccept(w) {
// 				// compute dot-product
// 				rawDP := w.CPU*node.TotalCPU + w.Memory*node.TotalMemory
// 				// compute CI score: TODO refine formula or fetch dynamic metrics
// 				rawCI := node.CarbonIntensity

// 				// combine costs: weight dp and ci
// 				costF := -rawDP + 0.1*rawCI // dp prioritized, CI penalizes
// 				cost := int(costF * 1000)

// 				log.Printf("   • Job %s->Node %s: DP=%.2f, CI=%.2f, cost=%d", w.ID, node.Name, rawDP, rawCI, cost)
// 				g.addEdge(workOff+i, nodeOff+j, 1, cost)
// 			}
// 		}
// 		// fallback unscheduled
// 		g.addEdge(workOff+i, unsched, 1, 0)
// 	}

// 	// machines -> sink
// 	for j := 0; j < m; j++ {
// 		g.addEdge(nodeOff+j, sink, 1, 0)
// 	}
// 	// unscheduled -> sink
// 	g.addEdge(unsched, sink, n, 0)

// 	// 2) Run MCFP
// 	flow, _ := g.minCostMaxFlow(src, sink)
// 	log.Printf("← scheduleBatch: MCFP assigned %d/%d jobs", flow, n)
// 	if flow == 0 {
// 		return
// 	}

// 	// extract assignments
// 	newPending := make([]core.Workload, 0, n)
// 	for i, w := range s.pending {
// 		assigned := false
// 		for _, e := range g.adj[workOff+i] {
// 			if e.to >= nodeOff && e.to < nodeOff+m && e.flow > 0 {
// 				j := e.to - nodeOff
// 				node := s.Nodes[j]

// 				log.Printf("   → Assign Job %s to Node %s (flow=%d)", w.ID, node.Name, e.flow)

// 				node.Reserve(w, s.Clock)
// 				s.timeline = append(s.timeline, Event{
// 					Time:     s.Clock.Add(w.Duration),
// 					Type:     End,
// 					Workload: w,
// 					Node:     node,
// 				})
// 				s.Logs = append(s.Logs,
// 					fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, node.Name, w.SubmitTime, s.Clock, s.Clock.Add(w.Duration)))
// 				assigned = true
// 				break
// 			}
// 		}
// 		if !assigned {
// 			newPending = append(newPending, w)
// 		}
// 	}
// 	s.pending = newPending
// }


// TODO placeholders until job-specific CPU/Memory tracked during release
// const (
// 	wPlaceholderCPU = 0.0
// 	wPlaceholderMem = 0.0
// )
