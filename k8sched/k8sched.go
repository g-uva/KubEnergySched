package k8sched

import (
	"log"
	"kube-scheduler/ecsched"
)

// K8Simulator simulates Kubernetes scheduling

type K8Simulator struct {
	inner *ecsched.DiscreteEventScheduler
}

// NewK8Simulator constructs a Kubernetes heuristic simulator
func NewK8Simulator(nodes []*ecsched.SimulatedNode) *K8Simulator {
	s := ecsched.NewScheduler(nodes)
	s.SchedType = ecsched.Kubernetes
	return &K8Simulator{inner: s}
}

// AddWorkload forwards arrival
func (s *K8Simulator) AddWorkload(w ecsched.Workload) {
	s.inner.AddWorkload(w)
}

// Run executes heuristic
func (s *K8Simulator) Run() {
	log.Print("[K8Simulator] running Kubernetes heuristic...")
	s.inner.Run()
}

// Logs exposes decisions
func (s *K8Simulator) Logs() []ecsched.LogEntry {
	return s.inner.Logs
}