package k8sched

import (
	"context"
	"math"
	"time"

	"kube-scheduler/pkg/core"
	"kube-scheduler/pkg/metrics"
)

type policy struct{}
func (p *policy) Name() string { return "k8" }
func (p *policy) Score(_ context.Context, w core.Workload, nodes []core.SimulatedNode) (core.Scores, error) {
	sc := core.Scores{}
	for _, n := range nodes {
		if !n.CanAccept(w) { continue }
		used := (n.TotalCPU-n.AvailableCPU)/n.TotalCPU + (n.TotalMemory-n.AvailableMemory)/n.TotalMemory
		sc[n.ID] = used
	}
	if len(sc) == 0 { sc[""] = math.Inf(1) }
	return sc, nil
}
func (p *policy) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }

type Simulator struct {
	base core.BaseSim
	pol  *policy
}

func NewK8sSimulator(nodes []*core.SimulatedNode) *Simulator {
	s := &Simulator{pol: &policy{}}
	s.base.Init(nodes)
	s.base.Select = func(w core.Workload, sims []*core.SimulatedNode) *core.SimulatedNode {
		view := make([]core.SimulatedNode, 0, len(sims))
		for _, n := range sims { view = append(view, *n) }
		sc, _ := s.pol.Score(context.Background(), core.Workload{
			ID: w.ID, CPU: w.CPU, Memory: w.Memory, Duration: w.Duration,
		}, view)
		if id, ok := s.pol.Select(sc); ok {
			for _, n := range sims {
				if n.Name == id && n.CanAccept(w) {
					start := s.base.Clock
					n.Reserve(w, start)
					end := start.Add(w.Duration)
					ci := metrics.ComputeCICost(n, core.Workload{
						ID: w.ID, CPU: w.CPU, Memory: w.Memory, Duration: w.Duration,
					}, start)
					s.base.LogsBuf = append(s.base.LogsBuf, core.LogEntry{
						JobID: w.ID, Node: n.Name,
						Submit: w.SubmitTime, Start: start, End: end,
						WaitMS: int64(start.Sub(w.SubmitTime)/time.Millisecond),
						CICost: ci,
					})
					return n
				}
			}
		}
		return nil
	}
	return s
}

// adapters
func (s *Simulator) SetScheduleBatchSize(n int)  { s.base.SetScheduleBatchSize(n) }
func (s *Simulator) AddWorkload(j core.Workload) { s.base.AddWorkload(j) }
func (s *Simulator) Run()                        { s.base.Run() }
func (s *Simulator) Logs() []core.LogEntry       { return s.base.Logs() }
