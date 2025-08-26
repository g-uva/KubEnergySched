// Package: models/cisched/cisched.go
package cisched

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"kube-scheduler/pkg/core"
	"kube-scheduler/pkg/metrics"
)

// Score implements the CI-Aware scorer with robust scaling and a soft util/queue guard.
// NOTE: We adapt Job -> Workload so CanAccept() (which expects Workload) works.
func (p *Policy) Score(_ context.Context, j core.Job, nodes []core.SimulatedNode) (core.Scores, error) {
	now := time.Now()

	// Job -> Workload adaptation (keeps your Job struct unchanged).
	w := core.Workload{
		ID:         j.ID,
		CPU:        j.CPUReq,
		Memory:     j.MemReq,
		Duration:   time.Duration(j.EstimatedDuration * float64(time.Second)),
		SubmitTime: j.SubmitAt,
		Labels:     j.Labels,
	}

	type feat struct {
		key         string
		ciCostG     float64
		waitSeconds float64
		utilOrQueue float64
		skip        bool
	}
	features := make([]feat, 0, len(nodes))

	for _, n := range nodes {
		if !n.CanAccept(w) {
			features = append(features, feat{skip: true})
			continue
		}

		// 1) Carbon impact (your ComputeCICost should already apply PUE*K and region CI).
		ci := metrics.ComputeCICost(&n, w, now) // grams CO₂

		// 2) Wait proxy (0 when free).
		waitS := 0.0
		if d := nextReleaseAfter(n, now); d > 0 {
			waitS = d.Seconds()
		}

		// 3) Soft guard: utilisation preferred; else queue length (both will be scaled).
		guard := utilisationOrQueue(n)

		features = append(features, feat{
			key:         nodeKey(n),
			ciCostG:     ci,
			waitSeconds: waitS,
			utilOrQueue: guard,
		})
	}

	// Collect for scaling (ignore skipped).
	var cis, waits, utils []float64
	for _, f := range features {
		if f.skip {
			continue
		}
		cis = append(cis, f.ciCostG)
		waits = append(waits, f.waitSeconds)
		utils = append(utils, f.utilOrQueue)
	}

	// Robust (5–95) or min–max fallback.
	scale := p.Scale
	if !scale.Enable {
		scale = RobustScalingCfg{Enable: false, QLow: 0.0, QHigh: 1.0, Eps: 1e-9}
	} else {
		if scale.QLow <= 0 || scale.QLow >= 0.5 {
			scale.QLow = 0.05
		}
		if scale.QHigh <= 0.5 || scale.QHigh >= 1.0 {
			scale.QHigh = 0.95
		}
		if scale.Eps == 0 {
			scale.Eps = 1e-9
		}
	}

	ciScaler := buildScaler(cis, scale)
	waitScaler := buildScaler(waits, scale)
	utilScaler := buildScaler(utils, scale)

	// Compose (lower is better).
	sc := core.Scores{} // map[string]float64

	for _, f := range features {
		if f.skip {
			continue
		}
		ciZ := ciScaler(f.ciCostG)
		waitZ := waitScaler(f.waitSeconds)
		utilZ := utilScaler(f.utilOrQueue)

		score := p.W.Carbon*ciZ + p.W.Wait*waitZ + p.W.Util*utilZ
		sc[f.key] = score
	}

	return sc, nil
}

// ----------------- helpers -----------------

func nodeKey(n core.SimulatedNode) string {
	if v, ok := any(n).(interface{ ID() string }); ok {
		return v.ID()
	}
	if v, ok := any(n).(interface{ Name() string }); ok {
		return v.Name()
	}
	return fmt.Sprintf("%p", &n)
}

func nextReleaseAfter(n core.SimulatedNode, t time.Time) time.Duration {
	if v, ok := any(n).(interface{ NextReleaseAfter(time.Time) time.Duration }); ok {
		return v.NextReleaseAfter(t)
	}
	return 0
}

func utilisationOrQueue(n core.SimulatedNode) float64 {
	if v, ok := any(n).(interface{ Utilisation() float64 }); ok {
		u := v.Utilisation()
		if !math.IsNaN(u) && !math.IsInf(u, 0) {
			return clamp(u, 0, 1)
		}
	}
	if v, ok := any(n).(interface{ QueueLen() int }); ok {
		q := float64(v.QueueLen())
		if q < 0 {
			q = 0
		}
		return q
	}
	return 0
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// buildScaler uses cisched.RobustScalingCfg (not core.*)
func buildScaler(vals []float64, cfg RobustScalingCfg) func(float64) float64 {
	clean := make([]float64, 0, len(vals))
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return func(x float64) float64 { return 0 }
	}
	sort.Float64s(clean)

	if cfg.Enable {
		qlo := percentile(clean, cfg.QLow)
		qhi := percentile(clean, cfg.QHigh)
		den := qhi - qlo
		if den < cfg.Eps {
			return func(x float64) float64 { return 0 }
		}
		return func(x float64) float64 { return clamp((x-qlo)/den, 0, 1) }
	}

	minV := clean[0]
	maxV := clean[len(clean)-1]
	den := maxV - minV
	if den < cfg.Eps {
		return func(x float64) float64 { return 0 }
	}
	return func(x float64) float64 { return clamp((x-minV)/den, 0, 1) }
}

func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}
