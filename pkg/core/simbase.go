package core

import (
	"math"
	"sort"
	"time"
)

type SelectFunc func(w Workload, nodes []*SimulatedNode) *SimulatedNode

type BaseSim struct {
	Clock   time.Time
	Nodes   []*SimulatedNode
	Batch   int
	Pending []Workload
	LogsBuf []LogEntry
	Select  SelectFunc
}

func (b *BaseSim) Init(nodes []*SimulatedNode) {
	b.Clock = time.Now()
	b.Nodes = nodes
	b.Batch = 1
	b.Pending = nil
	b.LogsBuf = nil
}

func (b *BaseSim) SetScheduleBatchSize(n int) { if n > 0 { b.Batch = n } }
func (b *BaseSim) AddWorkload(j Workload)     { b.Pending = append(b.Pending, j) }
func (b *BaseSim) Logs() []LogEntry           { return b.LogsBuf }

// simple eventless loop: process in submit-time order, greedy at current clock
func (b *BaseSim) Run() {
	sort.Slice(b.Pending, func(i, j int) bool { return b.Pending[i].SubmitTime.Before(b.Pending[j].SubmitTime) })
	queue := make([]Workload, 0, len(b.Pending))
	i := 0
	for i < len(b.Pending) || len(queue) > 0 {
		// advance time to next submit if idle
		if len(queue) == 0 && i < len(b.Pending) && b.Clock.Before(b.Pending[i].SubmitTime) {
			b.Clock = b.Pending[i].SubmitTime
		}
		// release resources at current time
		for _, n := range b.Nodes { n.Release(b.Clock) }
		// enqueue arrivals at/before now
		for i < len(b.Pending) && !b.Pending[i].SubmitTime.After(b.Clock) {
			queue = append(queue, b.Pending[i])
			i++
		}
		if len(queue) == 0 { continue }
		// schedule up to Batch
		next := queue[:0]
		scheduled := 0
		for _, w := range queue {
			if scheduled >= b.Batch { next = append(next, w); continue }
			n := b.selectNode(w)
			if n == nil { next = append(next, w); continue }
			start := b.Clock
			n.Reserve(w, start)
			end := start.Add(w.Duration)
			b.LogsBuf = append(b.LogsBuf, LogEntry{
				JobID:  w.ID, Node: n.Name, Submit: w.SubmitTime, Start: start, End: end,
				WaitMS: int64(start.Sub(w.SubmitTime) / time.Millisecond),
				// CICost: fill by caller if needed post-hoc, or compute inside SelectFunc if you prefer.
			})
			scheduled++
		}
		queue = next
		// advance time to the earliest reservation end (approx: pick smallest future end from nodes)
		earliest := time.Time{}
		for _, n := range b.Nodes {
			if t := n.NextReleaseAfter(b.Clock); !t.IsZero() { // implement as needed; else approximate
				if earliest.IsZero() || t.Before(earliest) { earliest = t }
			}
		}
		if earliest.IsZero() {
			// fallback small tick to avoid stalling if NextReleaseAfter not implemented
			earliest = b.Clock.Add(1 * time.Second)
		}
		b.Clock = earliest
	}
}

// default least-loaded fallback if no Select provided
func (b *BaseSim) selectNode(w Workload) *SimulatedNode {
	if b.Select != nil {
		if n := b.Select(w, b.Nodes); n != nil { return n }
	}
	var best *SimulatedNode
	bestScore := math.MaxFloat64
	for _, n := range b.Nodes {
		if !n.CanAccept(w) { continue }
		used := (n.TotalCPU-n.AvailableCPU)/n.TotalCPU + (n.TotalMemory-n.AvailableMemory)/n.TotalMemory
		if used < bestScore { bestScore, best = used, n }
	}
	return best
}
