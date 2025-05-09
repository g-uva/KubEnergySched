package main

import (
    "fmt"
    "runtime"

    "github.com/you/energy-scheduler/plugins"
    "k8s.io/apimachinery/pkg/runtime/schema"
    schedcfg "k8s.io/kube-scheduler/config/v1"
    "k8s.io/kube-scheduler/pkg/app"
    "k8s.io/kube-scheduler/pkg/framework/plugins/registry"
    "k8s.io/kubernetes/pkg/scheduler/framework"
)

func init() {
    registry.Register(registry.Plugin{Name: "EnergyEfficiencyPlugin", New: plugins.NewEnergy})
    registry.Register(registry.Plugin{Name: "DVFSPlugin", New: plugins.NewDVFS})
}

func main() {
    fmt.Printf("Starting Energy-aware Scheduler (%s/%s)\n", runtime.GOOS, runtime.GOARCH)
    opts := app.Options{
        ComponentConfig: &schedcfg.KubeSchedulerConfiguration{},
        ConfigFile:      "/etc/kube-scheduler/config.yaml",
    }
    app.Run(opts)
}
