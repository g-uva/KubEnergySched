package carbonscaler

import (
	"context"
	"time"

	"kube-scheduler/pkg/core"
)

type Config struct{ Lambda float64 }

type Policy struct{ Cfg Config }

func (p *Policy) Name() string { return "carbonscaler" }

func (p *Policy) Score(_ context.Context, j core.Job, nodes []core.SimulatedNode) (core.Scores, error) {
	// Job → Workload wrapper (so CanAccept works)
	w := core.Workload{
		ID:         j.ID,
		CPU:        j.CPUReq,
		Memory:     j.MemReq,
		Duration:   time.Duration(j.EstimatedDuration),
		SubmitTime: j.SubmitAt,
	}

	sc := core.Scores{}
	now := time.Now()

	for _, n := range nodes {
		if !n.CanAccept(w) { // uses Workload
			continue
		}
		usedCPU := 0.0
		if n.TotalCPU > 0 {
			usedCPU = (n.TotalCPU - n.AvailableCPU) / n.TotalCPU
		}
		usedMem := 0.0
		if n.TotalMemory > 0 {
			usedMem = (n.TotalMemory - n.AvailableMemory) / n.TotalMemory
		}
		utilPenalty := usedCPU + usedMem
		ci := n.CurrentCINorm(now) // assume normalised upstream (0..1)

		sc[n.Name] = utilPenalty + p.Cfg.Lambda*ci
	}
	return sc, nil
}

func (p *Policy) Select(sc core.Scores) (string, bool) { return core.ArgMin(sc) }


// Adapter methods
// func (s *Simulator) SetScheduleBatchSize(n int)  { s.base.SetScheduleBatchSize(n) }
// func (s *Simulator) AddWorkload(j core.Workload) { s.base.AddWorkload(j) }
// func (s *Simulator) Run()                        { s.base.Run() }
// func (s *Simulator) Logs() []core.LogEntry       { return s.base.Logs() }
