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

type SimulatedNode struct {
	Name            string
	AvailableCPU    float64
	AvailableMemory float64
	Scheduled       []Event
}

func (n *SimulatedNode) CanAccept(w Workload) bool {
	return n.AvailableCPU >= w.CPU && n.AvailableMemory >= w.Memory
}

func (n *SimulatedNode) Reserve(w Workload, now time.Time) {
	n.AvailableCPU -= w.CPU
	n.AvailableMemory -= w.Memory
	n.Scheduled = append(n.Scheduled, Event{
		Time:     now.Add(w.Duration),
		Type:     JobEnd,
		Workload: w,
		Node:     n,
	})
}

func (n *SimulatedNode) Release(w Workload) {
	n.AvailableCPU += w.CPU
	n.AvailableMemory += w.Memory
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
			evt.Node.Release(evt.Workload)
			log.Printf("Ended %s at %v on %s", evt.Workload.ID, s.Clock, evt.Node.Name)
		}
	}
}

func (s *DiscreteEventScheduler) handleArrival(w Workload) {
	for _, node := range s.Nodes {
		if node.CanAccept(w) {
			node.Reserve(w, s.Clock)
			s.Timeline = append(s.Timeline, Event{
				Time:     s.Clock.Add(w.Duration),
				Type:     JobEnd,
				Workload: w,
				Node:     node,
			})
			entry := fmt.Sprintf("%s,%s,%v,%v,%v", w.ID, node.Name, w.SubmitTime, s.Clock, s.Clock.Add(w.Duration))
			s.Logs = append(s.Logs, entry)
			log.Printf("Scheduled %s on %s at %v", w.ID, node.Name, s.Clock)
			return
		}
	}
	log.Printf("Job %s could not be scheduled", w.ID)
}
