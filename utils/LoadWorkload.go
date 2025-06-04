package utils

import (
	"encoding/csv"
	"os"
	"strconv"
	"kube-scheduler/central-unit"
    "crypto/rand"
    "encoding/hex"
    "path/filepath"
    "time"
    "fmt"
)

func LoadWorkloadsFromCSV(path string) ([]centralunit.Workload, error) {
    file, err := os.Open(path)
    if err != nil { return nil, err }
    defer file.Close()

    reader := csv.NewReader(file)
    rows, err := reader.ReadAll()
    if err != nil { return nil, err }

    var workloads []centralunit.Workload
    for _, row := range rows[1:] {
        cpu, _ := strconv.Atoi(row[1])
        ep, _ := strconv.ParseFloat(row[2], 64)
        workloads = append(workloads, centralunit.Workload{
            ID: row[0], CPURequirement: cpu, EnergyPriority: ep,
        })
    }
    return workloads, nil
}

func generateFilename() string {
    id := make([]byte, 6)
    if _, err := rand.Read(id); err != nil {
        panic("Failed to generate random ID: " + err.Error())
    }
    timestamp := time.Now().Format("20060102-140405")
    return fmt.Sprintf("results/%s_%s_benchmark.csv", hex.EncodeToString(id), timestamp)
}

func ensureResultsDirExit() {
    if err := os.MkdirAll("results", os.ModePerm); err != nil {
        panic("Failed to create results directory: " + err.Error())
    }
}
