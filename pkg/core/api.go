package core

type Node struct {
	ID      string
	CPUCap  float64
	MemCap  float64
	Metrics map[string]float64
}
type Job struct {
	ID         string
	CPUReq     float64
	MemReq     float64
	DeadlineMs int64
	Tags       map[string]string
}
type Scores map[string]float64

func ArgMin(sc Scores) (string, bool) {
	var best string
	bestV := 0.0
	first := true
	for id, v := range sc {
		if first || v < bestV { best, bestV, first = id, v, false }
	}
	return best, !first
}
