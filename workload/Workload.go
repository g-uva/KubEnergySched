package workload

import (
	"kube-scheduler/central-unit"
	"fmt"
	"math/rand"
)

func GenerateSyntheticWorkloads(num_jobs int) []centralunit.Workload {
	workloads := []centralunit.Workload{}
	for i := 0; i < num_jobs; i++ {
		w := centralunit.Workload{
			ID: fmt.Sprintf("job-%d", i),
			CPURequirement: rand.Intn(32) + 1, // Between 1 and 32 CPU cores
			EnergyPriority: rand.Float64(), // Between 0.0 and 1.0
		}
		workloads = append(workloads, w)
	}
	return workloads
}