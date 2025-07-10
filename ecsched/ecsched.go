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

type Reservation struct {
	Start  time.Time
	End    time.Time
	CPU    float64
	Memory float64
	JobID  string
}

type SimulatedNode struct {
	Name         string
	Reservations []Reservation
}

func (n *SimulatedNode) EarliestAvailable(w Workload, after time.Time) (time.Time, bool) {
	// Determine earliest time after 'after' when the node is free
	checkTime := after
	for _, r := range n.Reservations {
		if r.End.After(checkTime) {
			checkTime = r.End
		}
	}

	limit := checkTime.Add(1 * time.Hour) // Search window
	for checkTime.Before(limit) {
		if n.canFit(w, checkTime) {
			log.Printf("[%s] Found slot for %s at %v", n.Name, w.ID, checkTime)
			return checkTime, true
		}
		checkTime = checkTime.Add(5 * time.Second)
	}
	log.Printf("[%s] No slot found for %s in time window", n.Name, w.ID)
	return time.Time{}, false
}

func (n *SimulatedNode) canFit(w Workload, start time.Time) bool {
	end := start.Add(w.Duration)
	var usedCPU, usedMem float64
	for _, r := range n.Reservations {
		if r.End.After(start) && r.Start.Before(end) {
			usedCPU += r.CPU
			usedMem += r.Memory
		}
	}
	log.Printf("[%s] Checking canFit for %s at %v–%v | usedCPU=%.2f, usedMem=%.2f", n.Name, w.ID, start, end, usedCPU, usedMem)
	log.Printf("[%s] has %d reservations", n.Name, len(n.Reservations))
	return (usedCPU <= 16.0 - w.CPU) && (usedMem <= 32000.0 - w.Memory)
}

func (n *SimulatedNode) Reserve(w Workload, start time.Time) {
	n.Reservations = append(n.Reservations, Reservation{
		Start:  start,
		End:    start.Add(w.Duration),
		CPU:    w.CPU,
		Memory: w.Memory,
		JobID:  w.ID,
	})
	log.Printf("[%s] Reserving job %s: %v–%v (CPU=%.2f, MEM=%.2f)", n.Name, w.ID, start, start.Add(w.Duration), w.CPU, w.Memory)
	log.Printf("[%s] Reservations count: %d", n.Name, len(n.Reservations))
}

type DiscreteEventScheduler struct {
	Clock    time.Time
	Nodes    []*SimulatedNode
	Timeline []Event
	Logs     []string
}

func NewScheduler(nodes []*SimulatedNode) *DiscreteEventScheduler {
	return &DiscreteEventScheduler{
		Clock:    time.Now(),
		Nodes:    nodes,
		Timeline: []Event{},
	}
}

func (s *DiscreteEventScheduler) AddWorkload(w Workload) {
	s.Timeline = append(s.Timeline, Event{
		Time:     w.SubmitTime,
		Type:     JobArrival,
		Workload: w,
	})
}

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
			// log.Printf("Ended %s at %v on %s", evt.Workload.ID, s.Clock, evt.Node.Name)
		}
	}
}

func (s *DiscreteEventScheduler) handleArrival(w Workload) {
	type nodeOption struct {
		node  *SimulatedNode
		start time.Time
	}
	var options []nodeOption
	for _, node := range s.Nodes {
		if start, ok := node.EarliestAvailable(w, w.SubmitTime); ok {
			options = append(options, nodeOption{node: node, start: start})
		}
	}
	if len(options) == 0 {
		log.Printf("Job %s could not be scheduled", w.ID)
		return
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].start.Equal(options[j].start) {
			return options[i].node.Name < options[j].node.Name
		}
		return options[i].start.Before(options[j].start)
	})

	chosen := options[0]
	n := chosen.node
	start := chosen.start
	end := start.Add(w.Duration)

	n.Reserve(w, start)

	s.Timeline = append(s.Timeline, Event{
		Time:     end,
		Type:     JobEnd,
		Workload: w,
		Node:     n,
	})

	entry := fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, n.Name, w.SubmitTime.Format(time.RFC3339), start.Format(time.RFC3339), end.Format(time.RFC3339))
	s.Logs = append(s.Logs, entry)
	log.Printf("Scheduled %s on %s at %v", w.ID, n.Name, start)
}
