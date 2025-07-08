package core

type Workload struct {
	ID             string
	CPURequirement int
	EnergyPriority float64 // 0.0 to 1.0, higher = more energy aware
}
