package plugins

import (
    "context"
    "math"

    v1 "k8s.io/api/core/v1"
    "k8s.io/kubernetes/pkg/scheduler/framework"
)

type EnergyEfficiencyPlugin struct{}

func (pl *EnergyEfficiencyPlugin) Name() string { return "EnergyEfficiencyPlugin" }

func (pl *EnergyEfficiencyPlugin) Filter(ctx context.Context, _ *framework.CycleState, _ *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
    if nodeInfo.Allocatable.MilliCPU < 500 { // require ≥ 0.5 CPU
        return framework.NewStatus(framework.Unschedulable, "insufficient CPU")
    }
    return framework.NewStatus(framework.Success)
}

func (pl *EnergyEfficiencyPlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
    nodeInfo, err := state.NodeInfo(nodeName)
    if err != nil {
        return 0, framework.NewStatus(framework.Error, err.Error())
    }
    // util = (alloc − requested) / alloc
    util := float64(nodeInfo.Allocatable.MilliCPU-nodeInfo.Requested.MilliCPU) / float64(nodeInfo.Allocatable.MilliCPU)
    // Lin et al. coefficients :contentReference[oaicite:0]{index=0}:contentReference[oaicite:1]{index=1}
    c0, c1, c2 := 50.0, 100.0, 20.0
    d0, d1, d2 := 10.0, 50.0, 10.0
    power := d0 + d1*util + d2*util*util
    perf := c0 + c1*util + c2*util*util
    gEE := perf / power
    score := int64(math.Max(0, math.Min(100, gEE*100)))
    return score, framework.NewStatus(framework.Success)
}

func (pl *EnergyEfficiencyPlugin) ScoreExtensions() framework.ScoreExtensions {
    return nil
}
