package ecsched

import (
	"container/list"
	"log"
	"math"
	"time"

	"kube-scheduler/pkg/metrics"
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

// reservation tracks resources for proper release

type reservation struct {
	endTime     time.Time
	cpuReserved float64
	memReserved float64
}

// SimulatedNode represents a cluster node

type SimulatedNode struct {
	Name            string
	TotalCPU        float64
	TotalMemory     float64
	AvailableCPU    float64
	AvailableMemory float64
	CarbonIntensity float64           // gCO₂/kWh
	Reservations    *list.List        // holds *reservation entries
	Metadata        map[string]string // holds profiles, etc.
}

// CanAccept reports whether node has sufficient resources
func (n *SimulatedNode) CanAccept(w core.Workload) bool {
	return n.AvailableCPU >= w.CPU && n.AvailableMemory >= w.Memory
}

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

// EventType defines arrival or end

type EventType int

const (
	JobArrival EventType = iota
	JobEnd
)

// Event drives the discrete-event simulation

type Event struct {
	Time     time.Time
	Type     EventType
	Workload core.Workload
	Node     *core.SimulatedNode
}

// DiscreteEventScheduler drives simulation

type DiscreteEventScheduler struct {
	Clock             time.Time
	Nodes             []*core.SimulatedNode
	Events            []Event
	Logs              []LogEntry
	SchedType         SchedulerType
	CIBaseWeight      float64
	CIDynAlpha        float64
	ScheduleBatchSize int
	Pending           []core.Workload
}

// NewScheduler initializes with nodes and defaults

func NewScheduler(nodes []*core.SimulatedNode) *DiscreteEventScheduler {
	return &DiscreteEventScheduler{
		Clock:             time.Now(),
		Nodes:             nodes,
		Events:            []Event{},
		Logs:              []LogEntry{},
		SchedType:         MCFP,
		CIBaseWeight:      0.1,
		CIDynAlpha:        1.0,
		ScheduleBatchSize: 1,
		Pending:           []core.Workload{},
	}
}

// AddWorkload enqueues an arrival event

func (s *DiscreteEventScheduler) AddWorkload(w core.Workload) {
	s.Events = append(s.Events, Event{Time: w.SubmitTime, Type: JobArrival, Workload: w})
}

// Run executes all events and flushes final batch

func (s *DiscreteEventScheduler) Run() {
	s.sortEvents()
	for len(s.Events) > 0 {
		e := s.Events[0]
		s.Events = s.Events[1:]
		s.Clock = e.Time
		s.processReleases(s.Clock)
		s.handleEvent(e)
	}
	// flush any remaining pending jobs
	s.scheduleBatch()
}

// sortEvents keeps events time-ordered

func (s *DiscreteEventScheduler) sortEvents() {
	slice := s.Events
	for i := range slice {
		for j := i + 1; j < len(slice); j++ {
			if slice[j].Time.Before(slice[i].Time) {
				slice[i], slice[j] = slice[j], slice[i]
			}
		}
	}
}

// processReleases frees resources then backfills pending

func (s *DiscreteEventScheduler) processReleases(t time.Time) {
	for _, n := range s.Nodes {
		n.Release(t)
	}
	var still []core.Workload
	for _, w := range s.Pending {
		if node := s.selectNode(w); node != nil {
			node.Reserve(w, t)
			s.Events = append(s.Events, Event{Time: t.Add(w.Duration), Type: JobEnd, Node: node, Workload: w})
			ciCost := metrics.ComputeCICost(node, w, t)
			s.Logs = append(s.Logs, LogEntry{JobID: w.ID, Node: node.Name, Submit: w.SubmitTime, Start: t, End: t.Add(w.Duration), WaitMS: int64(t.Sub(w.SubmitTime)/time.Millisecond), CICost: ciCost})
		} else {
			still = append(still, w)
		}
	}
	s.Pending = still
}

// handleEvent schedules arrivals in batches or queues them

func (s *DiscreteEventScheduler) handleEvent(e Event) {
	switch e.Type {
	case JobArrival:
		s.Pending = append(s.Pending, e.Workload)
		if len(s.Pending) >= s.ScheduleBatchSize {
			s.scheduleBatch()
		}
	case JobEnd:
		log.Printf("Job %s ended on %s at %v", e.Workload.ID, e.Node.Name, s.Clock)
	}
}

// scheduleBatch builds a single MCFP and assigns many jobs

func (s *DiscreteEventScheduler) scheduleBatch() {
	n := len(s.Pending)
	if n == 0 {
		return
	}
	m := len(s.Nodes)
	// graph offsets
	src := 0
	jobOff := 1
	nodeOff := jobOff + n
	unsched := nodeOff + m
	sink := unsched + 1
	// create graph
	g := newGraph(sink + 1)
	// src->job edges
	for i := 0; i < n; i++ {
		g.addEdge(src, jobOff+i, 1, 0)
	}
	// job->node + fallback
	for i, w := range s.Pending {
		for j, node := range s.Nodes {
			if node.CanAccept(w) {
				rawDP := w.CPU*node.TotalCPU + w.Memory*node.TotalMemory
				rawCI := node.CarbonIntensity
				cost := int((-rawDP + s.CIBaseWeight*rawCI) * 1000)
				g.addEdge(jobOff+i, nodeOff+j, 1, cost)
			}
		}
		g.addEdge(jobOff+i, unsched, 1, 0)
	}
	// node->sink edges
	for j := 0; j < m; j++ {
		g.addEdge(nodeOff+j, sink, 1, 0)
	}
	// unscheduled->sink
	g.addEdge(unsched, sink, n, 0)
	// solve MCFP
	flow, _ := g.minCostMaxFlow(src, sink)
	if flow == 0 {
		return
	}
	// extract assignments
	var next []core.Workload
	for i, w := range s.Pending {
		assigned := false
		for _, e := range g.adj[jobOff+i] {
			if e.to >= nodeOff && e.to < nodeOff+m && e.flow > 0 {
				node := s.Nodes[e.to-nodeOff]
				t := s.Clock
				node.Reserve(w, t)
				s.Events = append(s.Events, Event{Time: t.Add(w.Duration), Type: JobEnd, Node: node, Workload: w})
				ciCost := metrics.ComputeCICost(node, w, t)
				s.Logs = append(s.Logs, LogEntry{JobID: w.ID, Node: node.Name, Submit: w.SubmitTime, Start: t, End: t.Add(w.Duration), WaitMS: int64(t.Sub(w.SubmitTime)/time.Millisecond), CICost: ciCost})
				assigned = true
				break
			}
		}
		if !assigned {
			next = append(next, w)
		}
	}
	s.Pending = next
}

// selectNode dispatches to the configured algorithm

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

// Reserve consumes capacity and records reservation entries

func (n *SimulatedNode) Reserve(w core.Workload, start time.Time) {
	n.AvailableCPU -= w.CPU
	n.AvailableMemory -= w.Memory
	n.Reservations.PushBack(&reservation{endTime: start.Add(w.Duration), cpuReserved: w.CPU, memReserved: w.Memory})
}

// Release frees resources for reservations ended <= t

func (n *SimulatedNode) Release(t time.Time) {
	for e := n.Reservations.Front(); e != nil; {
		next := e.Next()
		r := e.Value.(*reservation)
		if !r.endTime.After(t) {
			n.AvailableCPU = math.Min(n.AvailableCPU+r.cpuReserved, n.TotalCPU)
			n.AvailableMemory = math.Min(n.AvailableMemory+r.memReserved, n.TotalMemory)
			n.Reservations.Remove(e)
		}
		e = next
	}
}

// scheduleKubernetes: least-loaded heuristic
func (s *DiscreteEventScheduler) scheduleKubernetes(w core.Workload) *core.SimulatedNode {
	var best *core.SimulatedNode
	bestScore := math.MaxFloat64
	for _, n := range s.Nodes {
		if !n.CanAccept(w) {
			continue
		}
		used := (n.TotalCPU-n.AvailableCPU)/n.TotalCPU + (n.TotalMemory-n.AvailableMemory)/n.TotalMemory
		if used < bestScore {
			bestScore = used
			best = n
		}
	}
	return best
}

// scheduleSwarm: most-loaded heuristic
func (s *DiscreteEventScheduler) scheduleSwarm(w core.Workload) *core.SimulatedNode {
	var best *core.SimulatedNode
	bestScore := -1.0
	for _, n := range s.Nodes {
		if !n.CanAccept(w) {
			continue
		}
		ratio := (n.TotalCPU-n.AvailableCPU)/n.TotalCPU + (n.TotalMemory-n.AvailableMemory)/n.TotalMemory
		if ratio > bestScore {
			bestScore = ratio
			best = n
		}
	}
	return best
}

// scheduleMCFP: per-job dot-product + CI penalty fallback
func (s *DiscreteEventScheduler) scheduleMCFP(w core.Workload) *core.SimulatedNode {
	var sum, sumSq float64
	for _, n := range s.Nodes {
		sum += n.CarbonIntensity
		sumSq += n.CarbonIntensity * n.CarbonIntensity
	}
	mean := sum / float64(len(s.Nodes))
	variance := sumSq/float64(len(s.Nodes)) - mean*mean
	stddev := math.Sqrt(variance)
	dynW := s.CIBaseWeight * (1 + s.CIDynAlpha*(stddev/mean))
	var best *core.SimulatedNode
	bestCost := math.MaxFloat64
	for _, n := range s.Nodes {
		if !n.CanAccept(w) {
			continue
		}
		rawDP := w.CPU*n.TotalCPU + w.Memory*n.TotalMemory
		rawCI := n.CarbonIntensity
		cost := -rawDP + dynW*rawCI
		if cost < bestCost {
			bestCost = cost
			best = n
		}
	}
	return best
}

// --- MCFP Graph Implementation ---

type edge struct { to, rev, cap, cost, flow int }

type graph struct { adj [][]*edge }

// newGraph allocates a graph with n vertices
func newGraph(n int) *graph {
	g := &graph{adj: make([][]*edge, n)}
	return g
}

// addEdge adds directed edge u->v and its reverse
func (g *graph) addEdge(u, v, cap, cost int) {
	fwd := &edge{to: v, rev: len(g.adj[v]), cap: cap, cost: cost}
	bwd := &edge{to: u, rev: len(g.adj[u]), cap: 0, cost: -cost}
	g.adj[u] = append(g.adj[u], fwd)
	g.adj[v] = append(g.adj[v], bwd)
}

// minCostMaxFlow runs successive shortest path with potentials
func (g *graph) minCostMaxFlow(src, sink int) (int, int) {
	n := len(g.adj)
	INF := math.MaxInt32
	prevV := make([]int, n)
	prevE := make([]int, n)
	dist := make([]int, n)
	potential := make([]int, n)
	flow, cost := 0, 0
	for {
		// Dijkstra using potentials
		for i := 0; i < n; i++ { dist[i] = INF }
		dist[src] = 0
		inQ := make([]bool, n)
		queue := []int{src}
		inQ[src] = true
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			inQ[u] = false
			for i, e := range g.adj[u] {
				if e.cap > e.flow {
					next := e.to
					nd := dist[u] + e.cost + potential[u] - potential[next]
					if nd < dist[next] {
						dist[next] = nd
						prevV[next] = u
						prevE[next] = i
						if !inQ[next] {
							queue = append(queue, next)
							inQ[next] = true
						}
					}
				}
			}
		}
		if dist[sink] == INF {
			break
		}
		for v := 0; v < n; v++ {
			potential[v] += dist[v]
		}
		// find augmenting flow
		addf := INF
		for v := sink; v != src; v = prevV[v] {
			e := g.adj[prevV[v]][prevE[v]]
			if addf > e.cap-e.flow {
				addf = e.cap - e.flow
			}
		}
		for v := sink; v != src; v = prevV[v] {
			e := g.adj[prevV[v]][prevE[v]]
			e.flow += addf
			g.adj[v][e.rev].flow -= addf
			cost += addf * e.cost
		}
		flow += addf
	}
	return flow, cost
}
