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
    // 1) Get instantaneous grid CI (gCO₂/kWh)
    ci := currentCI(n, t)

    // 2) Estimate power draw (Watts)
    //    Assume node.Metadata["peak_power_w"] holds its max power in W
    pPeak, err := strconv.ParseFloat(n.Metadata["peak_power_w"], 64)
    if err != nil || pPeak <= 0 {
        // fallback to a default, e.g. 400 W
        pPeak = 400.0
    }
    // fraction of CPU it uses
    cpuFrac := w.CPU / n.TotalCPU

    // 3) Compute energy in kWh: (W × s) / 3600
    energyKWh := (pPeak * cpuFrac * w.Duration.Seconds()) / 3600.0

    // 4) Carbon cost = energy × CI
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
        // sine:<mean>:<amp>:<periodSec>
        mean, _ := strconv.ParseFloat(parts[1], 64)
        amp, _  := strconv.ParseFloat(parts[2], 64)
        period, _ := strconv.ParseInt(parts[3], 10, 64)
        θ := 2*math.Pi*float64(t.Unix()%period) / float64(period)
        return mean + amp*math.Sin(θ)
    case "randwalk":
        // assume inner scheduler has updated n.CarbonIntensity at each tick
        return n.CarbonIntensity
    default:
        // last‐resort: use whatever was in the node struct
        return n.CarbonIntensity
    }
}
