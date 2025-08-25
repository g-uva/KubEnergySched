package cisched

import (
	"context"
	"time"

	"kube-scheduler/pkg/core"
	"kube-scheduler/pkg/metrics"
)

type Weights struct {
	Carbon float64
	Wait   float64
	Queue  float64
	Price  float64
	Repro  float64
}
type Config struct{ W Weights }

type policy struct{ W Weights }
func (p *policy) Name() string { return "ci_aware" }

func (p *policy) Score(_ context.Context, w core.Workload, nodes []core.SimulatedNode) (core.Scores, error) {
	sc := core.Scores{}
	now := time.Now()
	for _, n := range nodes {
		if !n.CanAccept(w) { continue }
		ciCost := metrics.ComputeCICost(&n, w, now)
		util   := (w.CPU/(n.AvailableCPU+1e-9)) + (w.Memory/(n.AvailableMemory+1e-9))
		waitS  := 0.0
		if t := n.NextReleaseAfter(now); !t.IsZero() { waitS = t.Sub(now).Seconds() }
		score := p.W.Carbon*ciCost + p.W.Wait*waitS + p.W.Queue*util // simple proxy
		sc[n.ID] = score
	}
	return sc, nil
}
func (p *policy) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }

type Simulator struct {
	base   core.BaseSim
	pol    *policy
}

func NewCIScheduler(nodes []*core.SimulatedNode, cfg Config) *Simulator {
	s := &Simulator{pol: &policy{W: cfg.W}}
	s.base.Init(nodes)
	s.base.Select = func(w core.Workload, sims []*core.SimulatedNode) *core.SimulatedNode {
		// Policy works directly over SimulatedNode via adapters
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
