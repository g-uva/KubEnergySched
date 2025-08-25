package metrics

import (
    "math"
    "strconv"
    "strings"
    "time"
    "kube-scheduler/pkg/core"
)

// computeCICost estimates the grams of CO₂ emitted by running workload w
// on node n starting at time t. It uses:
//  1) a time-varying CI profile (static, sine-wave, or random-walk)
//  2) an energy model: node peak power × CPU share × duration
//  3) unit conversions (W→kWh, then × gCO₂/kWh)
func ComputeCICost(n *core.SimulatedNode, w core.Workload, t time.Time) float64 {
	ci := currentCI(n, t) // gCO2/kWh

	pPeak, err := strconv.ParseFloat(n.Metadata["peak_power_w"], 64)
	if err != nil || pPeak <= 0 {
		pPeak = 400.0
	}
	cpuFrac := 0.0
	if n.TotalCPU > 0 {
		cpuFrac = w.CPU / n.TotalCPU
	}
	energyKWh := (pPeak * cpuFrac * math.Max(w.Duration.Seconds(), 0.0)) / 3600.0
	return energyKWh * ci
}

// currentCI parses the node’s ci_profile metadata and returns the
// carbon intensity at time t (gCO₂/kWh). Supports:
//   static:<value>
//   sine:<mean>:<amp>:<periodSec>
//   randwalk:<min>:<max>:<stepSec>  (uses n.CarbonIntensity as last value)
func currentCI(n *core.SimulatedNode, t time.Time) float64 {
	prof := n.Metadata["ci_profile"]
	parts := strings.Split(prof, ":")
	switch parts[0] {
	case "static":
		v, _ := strconv.ParseFloat(parts[1], 64)
		return v
	case "sine":
		mean, _ := strconv.ParseFloat(parts[1], 64)
		amp, _ := strconv.ParseFloat(parts[2], 64)
		periodSec, _ := strconv.ParseInt(parts[3], 10, 64)
		if periodSec <= 0 {
			return mean
		}
		theta := 2 * math.Pi * float64(t.Unix()%periodSec) / float64(periodSec)
		return mean + amp*math.Sin(theta)
	case "randwalk":
		return n.CarbonIntensity
	default:
		return n.CarbonIntensity
	}
}

