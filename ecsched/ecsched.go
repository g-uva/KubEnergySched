package ecsched

import (
	"fmt"
	"log"
	"sort"
	"time"
)

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

type Workload struct {
	ID         string
	SubmitTime time.Time
	Duration   time.Duration
	CPU        float64
	Memory     float64
}

// Reservation tracks a single job reservation on a node
// with its time window and resource usage
type Reservation struct {
	Start  time.Time
	End    time.Time
	CPU    float64
	Memory float64
}

// SimulatedNode represents a compute node with capacity and reservations
type SimulatedNode struct {
	Name            string
	CapacityCPU     float64
	CapacityMemory  float64
	Reservations    []Reservation // active and queued reservations
}

// CanAcceptAt checks if the workload can be scheduled at the given start time
func (n *SimulatedNode) CanAcceptAt(w Workload, start time.Time) bool {
	tEnd := start.Add(w.Duration)
	// accumulate overlapping reservations
	usedCPU := 0.0
	usedMem := 0.0
	for _, r := range n.Reservations {
		// if r overlaps [start, tEnd)
		if r.Start.Before(tEnd) && r.End.After(start) {
			usedCPU += r.CPU
			usedMem += r.Memory
		}
	}
	// check capacity
	if usedCPU+w.CPU > n.CapacityCPU || usedMem+w.Memory > n.CapacityMemory {
		return false
	}
	return true
}

// ReserveAt books the workload on the node at the given start time
func (n *SimulatedNode) ReserveAt(w Workload, start time.Time) {
	res := Reservation{
		Start:  start,
		End:    start.Add(w.Duration),
		CPU:    w.CPU,
		Memory: w.Memory,
	}
	n.Reservations = append(n.Reservations, res)
}

func (n *SimulatedNode) Release(w Workload) {
	// release not used in this simulation model
}

// DiscreteEventScheduler manages scheduling events
// respecting resource and earliest-fit logic

type DiscreteEventScheduler struct {
	Clock    time.Time
	Nodes    []*SimulatedNode
	Timeline []Event
	Logs     []string
}

// NewScheduler creates a new scheduler with nodes (must set CapacityCPU/Memory)
func NewScheduler(nodes []*SimulatedNode) *DiscreteEventScheduler {
	return &DiscreteEventScheduler{
		Clock:    time.Now(),
		Nodes:    nodes,
		Timeline: nil,
		Logs:     nil,
	}
}

// AddWorkload enqueues a job arrival event
func (s *DiscreteEventScheduler) AddWorkload(w Workload) {
	s.Timeline = append(s.Timeline, Event{Time: w.SubmitTime, Type: JobArrival, Workload: w})
}

// Run processes all events in time order
func (s *DiscreteEventScheduler) Run() {
	for len(s.Timeline) > 0 {
		sort.Slice(s.Timeline, func(i, j int) bool {
			return s.Timeline[i].Time.Before(s.Timeline[j].Time)
		})
		evt := s.Timeline[0]
		s.Timeline = s.Timeline[1:]
		s.Clock = evt.Time

		switch evt.Type {
		case JobArrival:
			s.handleArrival(evt.Workload)
		case JobEnd:
			evt.Node.Release(evt.Workload)
				log.Printf("Ended %s at %v on %s", evt.Workload.ID, s.Clock, evt.Node.Name)
		}
	}
}

// handleArrival picks a node based on idle and earliest-start criteria
func (s *DiscreteEventScheduler) handleArrival(w Workload) {
	log.Printf("→ Arrival %s at %v", w.ID, s.Clock)

type option struct {
	node  *SimulatedNode
	start time.Time
}
var options []option

// compute earliest possible start for each node
for _, node := range s.Nodes {
	// check immediate fit
	if node.CanAcceptAt(w, s.Clock) {
		options = append(options, option{node: node, start: s.Clock})
		log.Printf("  • candidate %s: idle→start=%v", node.Name, s.Clock)
		continue
	}
	// otherwise scan reservation end times for next slot
	// sort reservations by end time
	slot := s.Clock
	sorted := make([]Reservation, len(node.Reservations))
	copy(sorted, node.Reservations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].End.Before(sorted[j].End) })
	for _, r := range sorted {
		// try after this reservation ends
		candidate := r.End
		if node.CanAcceptAt(w, candidate) {
			slot = candidate
			break
		}
	}
	options = append(options, option{node: node, start: slot})
	log.Printf("  • candidate %s: next-free→start=%v, queued=%d", node.Name, slot, len(node.Reservations))
}

// sort candidates: idle first, then earliest, then name
log.Printf("→ Decision point: comparing candidate nodes for job %s", w.ID)
sort.Slice(options, func(i, j int) bool {
	idleI := options[i].start.Equal(s.Clock)
	idleJ := options[j].start.Equal(s.Clock)
	if idleI != idleJ {
		return idleI
	}
	if options[i].start.Equal(options[j].start) {
		return options[i].node.Name < options[j].node.Name
	}
	return options[i].start.Before(options[j].start)
})
for _, opt := range options {
	log.Printf("   • %s: start=%v, queued=%d", opt.node.Name, opt.start, len(opt.node.Reservations))
}

// select winner
winner := options[0]
winner.node.ReserveAt(w, winner.start)

// enqueue job end event
s.Timeline = append(s.Timeline, Event{Time: winner.start.Add(w.Duration), Type: JobEnd, Workload: w, Node: winner.node})

// record log entry
entry := fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, winner.node.Name, w.SubmitTime, winner.start, winner.start.Add(w.Duration))
s.Logs = append(s.Logs, entry)
log.Printf("Scheduled %s on %s at %v", w.ID, winner.node.Name, winner.start)
}
