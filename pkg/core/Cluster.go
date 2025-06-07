package core

type Cluster interface {
	Name() string
	CanAccept(w Workload) bool
	EstimateEnergyCost(w Workload) float64
	SubmitJob(w Workload) error
	CarbonIntensity() float64
}

