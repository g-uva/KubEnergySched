package core

// type Workload struct {
// 	ID             string
// 	CPURequirement int
// 	EnergyPriority float64 // 0.0 to 1.0, higher = more energy aware
// }

type Workload struct {
    ID         string        `json:"id"`
    SubmitTime time.Time     `json:"submit_time"`
    Duration   time.Duration `json:"duration"`
    CPU        float64       `json:"cpu"`
    Memory     float64       `json:"memory"`
}
