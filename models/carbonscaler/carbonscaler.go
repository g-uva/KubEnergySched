package carbonscaler

import (
	"context"
	"time"

	"kube-scheduler/pkg/core"
	"kube-scheduler/pkg/metrics"
)

type CarbonScaler struct{ Lambda float64 }

func (s *CarbonScaler) Name() string { return "carbonscaler" }

func (s *CarbonScaler) Score(ctx context.Context, job core.Job, nodes []core.Node) (core.Scores, error) {
	scores := core.Scores{}
	for _, n := range nodes {
		util := (n.Metrics["cpu_used"]/n.CPUCap) + (n.Metrics["mem_used"]/n.MemCap)
		ci := n.Metrics["ci_norm"]
		scores[n.ID] = util / (1.0 + s.Lambda*ci)
	}
	return scores, nil
}

func (s *CarbonScaler) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }

type Config struct{ Lambda float64 }

type Simulator struct {
	base   core.BaseSim
	policy *CarbonScaler
}

func NewCarbonScaler(nodes []*core.SimulatedNode, cfg Config) *Simulator {
	s := &Simulator{policy: &CarbonScaler{Lambda: cfg.Lambda}}
	s.base.Init(nodes)

	// Plug policy into BaseSim
	s.base.Select = func(w core.Workload, sims []*core.SimulatedNode) *core.SimulatedNode {
		// Build view
		nodeViews := make([]core.Node, 0, len(sims))
		for _, n := range sims {
			nodeViews = append(nodeViews, core.Node{
				ID:     n.Name,
				CPUCap: n.TotalCPU, MemCap: n.TotalMemory,
				Labels: n.Labels,
				Metrics: map[string]float64{
					"cpu_used": (n.TotalCPU - n.AvailableCPU),
					"mem_used": (n.TotalMemory - n.AvailableMemory),
					"ci_norm":  n.CurrentCINorm(s.base.Clock),
				},
			})
		}
		scores, _ := s.policy.Score(context.Background(), core.Job{ID: w.ID}, nodeViews)
		if id, ok := s.policy.Select(scores); ok {
			for _, n := range sims {
				if n.Name == id && n.CanAccept(w) {
					// Reserve & log with CI cost
					start := s.base.Clock
					n.Reserve(w, start)
					end := start.Add(w.Duration)
					ciCost := metrics.ComputeCICost(n, w, start)

					s.base.LogsBuf = append(s.base.LogsBuf, core.LogEntry{
						JobID:  w.ID,
						Node:   n.Name,
						Submit: w.SubmitTime,
						Start:  start,
						End:    end,
						WaitMS: int64(start.Sub(w.SubmitTime) / time.Millisecond),
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
func (s *Simulator) SetScheduleBatchSize(n int) { s.base.SetScheduleBatchSize(n) }
func (s *Simulator) AddWorkload(j core.Workload) { s.base.AddWorkload(j) }
func (s *Simulator) Run()                         { s.base.Run() }
func (s *Simulator) Logs() []core.LogEntry        { return s.base.Logs() }
