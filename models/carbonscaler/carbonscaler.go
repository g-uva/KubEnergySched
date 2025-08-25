package carbonscaler

import (
	"context"

	"kube-scheduler/pkg/core"
	"kube-scheduler/pkg/metrics"
)

type Config struct{ Lambda float64 } // weight for carbon term

type CarbonScaler struct{ cfg Config }

func (s *CarbonScaler) Name() string { return "carbonscaler" }

// POLICY: score on lightweight Node view (no mutation, no CanAccept here)
func (s *CarbonScaler) Score(_ context.Context, job core.Job, nodes []core.Node) (core.Scores, error) {
	sc := core.Scores{}
	for _, n := range nodes {
		usedCPU := 0.0
		if n.CPUCap > 0 {
			usedCPU = n.Metrics["cpu_used"] / n.CPUCap
		}
		usedMem := 0.0
		if n.MemCap > 0 {
			usedMem = n.Metrics["mem_used"] / n.MemCap
		}
		utilPenalty := usedCPU + usedMem
		ci := n.Metrics["ci_norm"] // 0..1 (provided by adapter)
		sc[n.ID] = utilPenalty + s.cfg.Lambda*ci
	}
	return sc, nil
}

func (s *CarbonScaler) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }

// SIMULATOR ADAPTER
type Simulator struct {
	base   core.BaseSim
	policy *CarbonScaler
}

func NewCarbonScaler(nodes []*core.SimulatedNode, cfg Config) *Simulator {
	s := &Simulator{policy: &CarbonScaler{cfg: cfg}}
	s.base.Init(nodes)

	s.base.Select = func(w core.Workload, sims []*core.SimulatedNode) *core.SimulatedNode {
		// Build view for policy
		v := make([]core.Node, 0, len(sims))
		for _, n := range sims {
			v = append(v, core.Node{
				ID:     n.Name,
				CPUCap: n.TotalCPU,
				MemCap: n.TotalMemory,
				Labels: n.Labels,
				Metrics: map[string]float64{
					"cpu_used": (n.TotalCPU - n.AvailableCPU),
					"mem_used": (n.TotalMemory - n.AvailableMemory),
					"ci_norm":  n.CurrentCINorm(s.base.Clock),
				},
			})
		}
		scores, _ := s.policy.Score(context.Background(), core.Job{ID: w.ID}, v)
		if id, ok := s.policy.Select(scores); ok {
			for _, n := range sims {
				if n.Name == id && n.CanAccept(w) {
					start := s.base.Clock
					n.Reserve(w, start)
					end := start.Add(w.Duration)
					ciCost := metrics.ComputeCICost(n, core.Workload{
						ID:                w.ID,
						CPU:               w.CPU,
						Memory:            w.Memory,
						Duration:          w.Duration,
					}, start)

					s.base.LogsBuf = append(s.base.LogsBuf, core.LogEntry{
						JobID:  w.ID,
						Node:   n.Name,
						Submit: w.SubmitTime,
						Start:  start,
						End:    end,
						WaitMS: int64(start.Sub(w.SubmitTime).Milliseconds()),
						CICost: ciCost,
					})
					return n
				}
			}
		}
		return nil
	}
	return s
}

// Adapter methods
func (s *Simulator) SetScheduleBatchSize(n int)  { s.base.SetScheduleBatchSize(n) }
func (s *Simulator) AddWorkload(j core.Workload) { s.base.AddWorkload(j) }
func (s *Simulator) Run()                        { s.base.Run() }
func (s *Simulator) Logs() []core.LogEntry       { return s.base.Logs() }
