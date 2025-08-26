package cisched

import (
	"context"

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

type Policy struct{ W Weights }

func (p *Policy) Name() string { return "ci_aware" }

// Score over lightweight Node views (no sim-only methods here)
func (p *Policy) Score(_ context.Context, j core.Job, nodes []core.Node) (core.Scores, error) {
	sc := core.Scores{}
	_ = j // reserved for future features (e.g., site prefs)
	for _, n := range nodes {
		// carbon proxy from adapter-provided ci_norm (0..1)
		ci := n.Metrics["ci_norm"]

		// simple utilisation proxy (0..2)
		util := 0.0
		if n.CPUCap > 0 {
			util += n.Metrics["cpu_used"] / n.CPUCap
		}
		if n.MemCap > 0 {
			util += n.Metrics["mem_used"] / n.MemCap
		}

		// wait proxy unavailable in Node view → set to 0 (adapter enforces feasibility)
		waitS := 0.0

		score := p.W.Carbon*ci + p.W.Wait*waitS + p.W.Queue*util
		sc[n.ID] = score
	}
	return sc, nil
}

func (p *Policy) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }

type Simulator struct {
	base   core.BaseSim
	policy *Policy
}

func NewCIScheduler(nodes []*core.SimulatedNode, cfg Config) *Simulator {
	s := &Simulator{policy: &Policy{W: cfg.W}}
	s.base.Init(nodes)

	s.base.Select = func(w core.Workload, sims []*core.SimulatedNode) *core.SimulatedNode {
		now := s.base.Clock

		// 1) Build Node views for policy
		view := make([]core.Node, 0, len(sims))
		for _, n := range sims {
			view = append(view, core.Node{
				ID:     n.Name,
				CPUCap: n.TotalCPU,
				MemCap: n.TotalMemory,
				Labels: n.Labels,
				Metrics: map[string]float64{
					"cpu_used": (n.TotalCPU - n.AvailableCPU),
					"mem_used": (n.TotalMemory - n.AvailableMemory),
					"ci_norm":  n.CurrentCINorm(now),
				},
			})
		}

		// 2) Workload -> Job view for scoring
		j := core.JobView(w)

		// 3) Score & select
		scores, _ := s.policy.Score(context.Background(), j, view)
		if id, ok := s.policy.Select(scores); ok {
			for _, n := range sims {
				if n.Name == id && n.CanAccept(w) {
					start := now
					n.Reserve(w, start)
					end := start.Add(w.Duration)

					// true CI impact (with PUE/k) for logging
					ci := metrics.ComputeCICost(n, w, start)

					s.base.LogsBuf = append(s.base.LogsBuf, core.LogEntry{
						JobID:  w.ID,
						Node:   n.Name,
						Submit: w.SubmitTime,
						Start:  start,
						End:    end,
						WaitMS: int64(start.Sub(w.SubmitTime).Milliseconds()),
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
