package benchmark

import (
    "encoding/csv"
    "fmt"
    "os"
    "time"
    "reflect"
    "kube-scheduler/central-unit"
    "encoding/hex"
    "crypto/rand"
)

// BenchmarkRecord holds detailed benchmark logs
type BenchmarkRecord struct {
    Timestamp       string
    WorkloadID      string
    StrategyName    string
    SelectedCluster string
    EstimatedCost   float64
    CarbonIntensity float64
    Reasoning       string
}

// BenchmarkAdapter adapts workloads into benchmarking runs
type BenchmarkAdapter struct {
    Clusters   []centralunit.Cluster
    Strategies []centralunit.SchedulingStrategy
    Workloads  []centralunit.Workload
    Results    []BenchmarkRecord
}

func (ba *BenchmarkAdapter) RunBenchmark() {
    for _, strategy := range ba.Strategies {
        fmt.Printf("\n[Benchmark] Running with strategy: %s\n", reflect.TypeOf(strategy).Name())
        cu := centralunit.CentralUnit{Clusters: ba.Clusters, Strategy: strategy}
        for _, w := range ba.Workloads {
            selected, reason, err := cu.Strategy.SelectCluster(cu.Clusters, w)
            if err != nil {
                fmt.Printf("Failed to schedule %s: %v\n", w.ID, err)
                continue
            }
            selected.SubmitJob(w)
            record := BenchmarkRecord{
                Timestamp:       time.Now().Format(time.RFC3339),
                WorkloadID:      w.ID,
                StrategyName:    reflect.TypeOf(strategy).Name(),
                SelectedCluster: selected.Name(),
                EstimatedCost:   selected.EstimateEnergyCost(w),
                CarbonIntensity: selected.CarbonIntensity(),
                Reasoning:       reason,
            }
            ba.Results = append(ba.Results, record)
        }
    }
}

func (ba *BenchmarkAdapter) ExportToCSV() error {
    
    // Ensuring that the results directory exists :)
    if err := os.MkdirAll("results", os.ModePerm); err != nil {
        panic("Failed to create results directory: " + err.Error())
    }

    filename := generateFilename()

    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    // Write headers
    headers := []string{"Timestamp", "WorkloadID", "Strategy", "Cluster", "EnergyCost", "CarbonIntensity", "Reasoning"}
    writer.Write(headers)

    for _, r := range ba.Results {
        row := []string{
            r.Timestamp,
            r.WorkloadID,
            r.StrategyName,
            r.SelectedCluster,
            fmt.Sprintf("%.2f", r.EstimatedCost),
            fmt.Sprintf("%.2f", r.CarbonIntensity),
            r.Reasoning,
        }
        writer.Write(row)
    }
    fmt.Printf("[Benchmark] Exported to CSV: %s\n", filename)
    return nil
}

func generateFilename() string {
    id := make([]byte, 6)
    if _, err := rand.Read(id); err != nil {
        panic("Failed to generate random ID: " + err.Error())
    }
    timestamp := time.Now().Format("20060102-140405")
    return fmt.Sprintf("results/%s_%s_benchmark.csv", hex.EncodeToString(id), timestamp)
}
