package ecsched

import (
	"container/heap"
	"fmt"
	"log"
	"sort"
	"time"
)

// -------------------- Node & Workload --------------------

type Node struct {
	Name             string
	TotalCPU         float64
	TotalMemory      float64
	AvailableCPU     float64
	AvailableMemory  float64
	CarbonIntensity  float64    // gCO₂/kWh, TODO: fetch from API
	Reservations     []time.Time
}

func NewNode(name string, cpu, mem, ci float64) *Node {
	return &Node{
		Name:            name,
		TotalCPU:        cpu,
		TotalMemory:     mem,
		AvailableCPU:    cpu,
		AvailableMemory: mem,
		CarbonIntensity: ci,
		Reservations:    nil,
	}
}

// CanAccept returns true if free resources suffice
func (n *Node) CanAccept(w Workload) bool {
	return n.AvailableCPU >= w.CPU && n.AvailableMemory >= w.Memory
}

// Reserve allocates resources and records end-time
func (n *Node) Reserve(w Workload, at time.Time) {
	n.AvailableCPU -= w.CPU
	n.AvailableMemory -= w.Memory
	n.Reservations = append(n.Reservations, at.Add(w.Duration))
}

// ReleaseExpired frees resources whose end-time <= now
func (n *Node) ReleaseExpired(now time.Time) {
	keep := n.Reservations[:0]
	for _, t := range n.Reservations {
		if t.After(now) {
			keep = append(keep, t)
		} // expired times assumed auto-restored via End events
	}
	n.Reservations = keep
}

// -------------------- Workload & Event --------------------

type Workload struct {
	ID         string
	SubmitTime time.Time
	Duration   time.Duration
	CPU        float64
	Memory     float64
}

type EventType int

const (
	Arrival EventType = iota
	End
)

type Event struct {
	Time     time.Time
	Type     EventType
	Workload Workload
	Node     *Node
}

// -------------------- Scheduler --------------------

type Scheduler struct {
	Clock    time.Time
	Nodes    []*Node
	timeline []Event    // pending events sorted by Time
	pending  []Workload // batch of arrivals waiting assignment
	Logs     []string   // "job,node,submit,start,end"
}

// NewScheduler constructs with given nodes
func NewScheduler(nodes []*Node) *Scheduler {
	return &Scheduler{Clock: time.Now(), Nodes: nodes}
}

// AddWorkload schedules an arrival event
func (s *Scheduler) AddWorkload(w Workload) {
	s.timeline = append(s.timeline, Event{Time: w.SubmitTime, Type: Arrival, Workload: w})
}

// Run processes all events
func (s *Scheduler) Run() {
	for len(s.timeline) > 0 {
		s.sortTimeline()
		e := s.timeline[0]
		s.timeline = s.timeline[1:]
		s.Clock = e.Time

		// release any expired at now
		for _, n := range s.Nodes {
			n.ReleaseExpired(s.Clock)
		}

		switch e.Type {
		case Arrival:
			s.pending = append(s.pending, e.Workload)
			s.scheduleBatch()

		case End:
			e.Node.AvailableCPU += e.Workload.CPU
			e.Node.AvailableMemory += e.Workload.Memory
			log.Printf("Ended %s at %v on %s", e.Workload.ID, s.Clock, e.Node.Name)
		}
	}
}

// sortTimeline orders events by Time ascending
func (s *Scheduler) sortTimeline() {
	sort.Slice(s.timeline, func(i, j int) bool {
		return s.timeline[i].Time.Before(s.timeline[j].Time)
	})
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

// -------------------- MCFP Implementation --------------------

type edge struct {
	to, rev, cap, cost, flow int
}

type graph struct {
	n   int
	adj [][]edge
}

func newGraph(n int) *graph {
	return &graph{n: n, adj: make([][]edge, n)}
}

func (g *graph) addEdge(u, v, cap, cost int) {
	g.adj[u] = append(g.adj[u], edge{to: v, rev: len(g.adj[v]), cap: cap, cost: cost})
	g.adj[v] = append(g.adj[v], edge{to: u, rev: len(g.adj[u]) - 1, cap: 0, cost: -cost})
}

func (g *graph) minCostMaxFlow(s, t int) (int, int) {
	N := g.n
	const INF = int(1e9)
	pot := make([]int, N)
	flow, flowCost := 0, 0
	for {
		dist := make([]int, N)
		prevV := make([]int, N)
		prevE := make([]int, N)
		for i := range dist {
			dist[i] = INF
		}
		dist[s] = 0

		hq := &intHeap{}
		heap.Init(hq)
		heap.Push(hq, heapItem{v: s, dist: 0})

		for hq.Len() > 0 {
			it := heap.Pop(hq).(heapItem)
			if it.dist > dist[it.v] {
				continue
			}
			for ei, e := range g.adj[it.v] {
				if e.cap > e.flow {
					rc := e.cost + pot[it.v] - pot[e.to]
					if nd := dist[it.v] + rc; nd < dist[e.to] {
						dist[e.to] = nd
						prevV[e.to] = it.v
						prevE[e.to] = ei
						heap.Push(hq, heapItem{v: e.to, dist: nd})
					}
				}
			}
		}
		if dist[t] == INF {
			break
		}
		for i := 0; i < N; i++ {
			if dist[i] < INF {
				pot[i] += dist[i]
			}
		}
		// augment one unit
		df := 1
		for v := t; v != s; v = prevV[v] {
			e := &g.adj[prevV[v]][prevE[v]]
			if df > e.cap-e.flow {
				df = e.cap - e.flow
			}
		}
		if df == 0 {
			break
		}
		for v := t; v != s; v = prevV[v] {
			e := &g.adj[prevV[v]][prevE[v]]
			e.flow += df
			g.adj[v][e.rev].flow -= df
			flowCost += df * e.cost
		}
		flow += df
	}
	return flow, flowCost
}

type heapItem struct { v, dist int }

type intHeap []heapItem

func (h intHeap) Len() int            { return len(h) }
func (h intHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h intHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *intHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *intHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
