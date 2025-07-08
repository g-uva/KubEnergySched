package trace_features

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"
)

type Workload struct {
    ID         string        `json:"id"`
    SubmitTime time.Time     `json:"submit_time"`
    Duration   time.Duration `json:"duration"`
    CPU        float64       `json:"cpu"`
    Memory     float64       `json:"memory"`
}

func main() {
    file, err := os.Open("data/powertrace.csv")
    if err != nil {
        log.Fatalf("Failed to open CSV: %v", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    rows, err := reader.ReadAll()
    if err != nil {
        log.Fatalf("Failed to read CSV: %v", err)
    }

    startTime := time.Now()
    var workloads []Workload

    for i, row := range rows {
        if i == 0 {
            continue // skip header
        }

        timeVal, _ := strconv.ParseInt(row[0], 10, 64)
        productionPower, _ := strconv.ParseFloat(row[4], 64)

        offset := time.Duration(timeVal) * time.Microsecond
        submit := startTime.Add(offset)
        cpu := productionPower * 4.0
        mem := productionPower * 8192.0

        workloads = append(workloads, Workload{
            ID:         fmt.Sprintf("trace-%d", i),
            SubmitTime: submit,
            Duration:   5 * time.Minute,
            CPU:        cpu,
            Memory:     mem,
        })
    }

    scheduleAndSend(workloads)
}

func scheduleAndSend(workloads []Workload) {
    for _, w := range workloads {
        // Optionally add a planning step here
        planned := planSchedule(w)

        body, err := json.Marshal(planned)
        if err != nil {
            log.Printf("Failed to encode workload %s: %v", w.ID, err)
            continue
        }

        resp, err := http.Post("http://centralunit.eu-central.svc.cluster.local:8080/handleWorkloadIngest", "application/json", bytes.NewReader(body))
        if err != nil {
            log.Printf("Failed to POST workload %s: %v", w.ID, err)
            continue
        }
        resp.Body.Close()

        log.Printf("Sent workload %s", w.ID)
    }
}

// PlanSchedule is a placeholder for your strategy
func planSchedule(w Workload) Workload {
    // In a real planner, you might offset start times, group loads, etc.
    return w
}
