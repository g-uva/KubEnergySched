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
	log.Printf("[%s] → Checking availability for %s after %v", n.Name, w.ID, after)
	log.Printf("[%s] → %d reservations", n.Name, len(n.Reservations))

	checkTime := after.Truncate(time.Second)
	limit := checkTime.Add(1 * time.Hour)
	for checkTime.Before(limit) {
		if n.canFit(w, checkTime) {
			log.Printf("[%s] ✅ canFit at %v", n.Name, checkTime)
			return checkTime, true
		} else {
			log.Printf("[%s] ❌ cannot fit at %v", n.Name, checkTime)
		}
		checkTime = checkTime.Add(5 * time.Second)
	}
	log.Printf("[%s] ❌ No fit for %s in time window", n.Name, w.ID)
	return time.Time{}, false
}

func (n *SimulatedNode) NextFreeAt() time.Time {
	var latest time.Time
	for _, r := range n.Reservations {
		if r.End.After(latest) {
			latest = r.End
		}
	}
	return latest.Truncate(time.Second)
}

func (n *SimulatedNode) canFit(w Workload, start time.Time) bool {
	start = start.Truncate(time.Second)
	end := start.Add(w.Duration).Truncate(time.Second)
	var usedCPU, usedMem float64
	log.Printf("[%s]   ↪ Reservation overlap check: job %s wants %v–%v", n.Name, w.ID, start, end)
	recent := n.Reservations
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	for _, r := range recent {
		log.Printf("[%s]     × Existing: %s from %v to %v", n.Name, r.JobID, r.Start, r.End)
		if r.End.After(start) && r.Start.Before(end) {
			usedCPU += r.CPU
			usedMem += r.Memory
		}
	}
	return (w.CPU <= 16.0 - usedCPU) && (w.Memory <= 32000.0 - usedMem)
}

func (n *SimulatedNode) Reserve(w Workload, start time.Time) {
	start = start.Truncate(time.Second)
	end := start.Add(w.Duration).Truncate(time.Second)
	n.Reservations = append(n.Reservations, Reservation{
		Start:  start,
		End:    end,
		CPU:    w.CPU,
		Memory: w.Memory,
		JobID:  w.ID,
	})
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
			// Do nothing for now
		}
	}
}

func (s *DiscreteEventScheduler) handleArrival(w Workload) {
	type nodeOption struct {
		node  *SimulatedNode
		start time.Time
	}
	var immediate []nodeOption
	var fallback []nodeOption
	for _, node := range s.Nodes {
		start, ok := node.EarliestAvailable(w, w.SubmitTime)
		log.Printf("[%s] EarliestAvailable for job %s: %v (ok=%v)", node.Name, w.ID, start, ok)
		if ok {
			log.Printf("[%s] → Added to immediate options", node.Name)
			immediate = append(immediate, nodeOption{node: node, start: start})
		} else {
			deferredStart := node.NextFreeAt()
			log.Printf("[%s] → No immediate fit. NextFreeAt: %v", node.Name, deferredStart)
			fallback = append(fallback, nodeOption{node: node, start: deferredStart})
		}
	}

	var options []nodeOption
	if len(immediate) > 0 {
		options = immediate
	} else {
		options = fallback
	}

	log.Printf("→ Final options for job %s:", w.ID)
	for _, opt := range options {
		log.Printf("  - %s: start at %v", opt.node.Name, opt.start)
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].start.Equal(options[j].start) {
			return options[i].node.Name < options[j].node.Name
		}
		return options[i].start.Before(options[j].start)
	})

	chosen := options[0]
	n := chosen.node
	start := chosen.start.Truncate(time.Second)
	end := start.Add(w.Duration).Truncate(time.Second)

	log.Printf("✔ Job %s assigned to %s at %v", w.ID, n.Name, start)

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
