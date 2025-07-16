package cisched

import (
	"log"
	"time"
	"kube-scheduler/ecsched"
	"kube-scheduler/pkg/core"
)

// CIScheduler wraps DiscreteEventScheduler for CI-awareness
// TODO: inject actual CI client

type CIScheduler struct {
	inner *ecsched.DiscreteEventScheduler
}

// NewCIScheduler builds a CI-aware baseline (using MCFP)
func NewCIScheduler(nodes []*core.SimulatedNode) *CIScheduler {
	s := ecsched.NewScheduler(nodes)
	return &CIScheduler{inner: s}
}

// AddWorkload forwards arrival
func (s *CIScheduler) AddWorkload(w core.Workload) {
	s.inner.AddWorkload(w)
}

// Run fetches CI metrics (TODO) then executes scheduling
func (s *CIScheduler) Run() {
	log.Print("[CIScheduler] fetching CI metrics... (TODO)")
	time.Sleep(10 * time.Millisecond)
	s.inner.Run()
}

func (s *CIScheduler) SetScheduleBatchSize(size int) {
	s.inner.ScheduleBatchSize = size
}

func (s *CIScheduler) SetCIBaseWeight(weight float64) {
	s.inner.CIBaseWeight = weight
}

// Logs exposes scheduling decisions
func (s *CIScheduler) Logs() []ecsched.LogEntry {
	return s.inner.Logs
}	