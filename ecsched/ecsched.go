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

// we no longer track CPU/Memory at all
type SimulatedNode struct {
	Name         string
	Reservations []Event      // only used to find next‐free time
}

func (n *SimulatedNode) ReserveAt(w Workload, start time.Time) {
	// record when this job will finish on this node
	n.Reservations = append(n.Reservations, Event{
		Time:     start.Add(w.Duration),
		Type:     JobEnd,
		Workload: w,
		Node:     n,
	})
}

func (n *SimulatedNode) Release(w Workload) {
	// no resources to free
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
		Timeline: nil,
		Logs:     nil,
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
			evt.Node.Release(evt.Workload)
			log.Printf("Ended %s at %v on %s", evt.Workload.ID, s.Clock, evt.Node.Name)
		}
	}
}

func (s *DiscreteEventScheduler) handleArrival(w Workload) {
	log.Printf("→ Arrival %s at %v", w.ID, s.Clock)

	type option struct {
		node  *SimulatedNode
		start time.Time
	}
	var options []option

	// 1) for each node, compute earliest start
	for _, node := range s.Nodes {
		var start time.Time
		if len(node.Reservations) == 0 {
			start = s.Clock
		} else {
			// find the latest reservation end among this node's queue
			latest := node.Reservations[0].Time
			for _, r := range node.Reservations {
				if r.Time.After(latest) {
					latest = r.Time
				}
			}
			start = latest
		}
		options = append(options, option{node: node, start: start})
		log.Printf("  • candidate %s: next‐free at %v, #queued=%d",
			node.Name, start, len(node.Reservations))
	}

	// 2) choose idle first, then earliest start, then name
	log.Printf("→ Decision point: comparing candidate nodes for job %s", w.ID)
	sort.Slice(options, func(i, j int) bool {
		idleI := len(options[i].node.Reservations) == 0
		idleJ := len(options[j].node.Reservations) == 0
		if idleI != idleJ {
			return idleI // idle before non‐idle
		}
		if options[i].start.Equal(options[j].start) {
			return options[i].node.Name < options[j].node.Name
		}
		return options[i].start.Before(options[j].start)
	})
	for _, opt := range options {
		log.Printf("   • %s: start=%v, queued=%d",
			opt.node.Name, opt.start, len(opt.node.Reservations))
	}

	// 3) schedule on the winner
	winner := options[0]
	winner.node.ReserveAt(w, winner.start)

	// enqueue its end event
	s.Timeline = append(s.Timeline, Event{
		Time:     winner.start.Add(w.Duration),
		Type:     JobEnd,
		Workload: w,
		Node:     winner.node,
	})

	// record to Logs
	entry := fmt.Sprintf("%s,%s,%v,%v,%v",
		w.ID, winner.node.Name,
		w.SubmitTime,
		winner.start,
		winner.start.Add(w.Duration),
	)
	s.Logs = append(s.Logs, entry)
	log.Printf("Scheduled %s on %s at %v", w.ID, winner.node.Name, winner.start)
}
