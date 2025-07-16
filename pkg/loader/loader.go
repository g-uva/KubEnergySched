package loader

import (
    "encoding/csv"
    "io"
    "log"
    "os"
    "strconv"
    "strings"
    "time"

    "kube-scheduler/pkg/core"
)

// loadNodesFromCSV parses a CSV of:
//
//   name,cpu,mem,ci_profile
//
// where ci_profile can be:
//   - "static:<value>"
//   - "sine:<mean>:<amp>:<periodSec>"
//   - "randwalk:<min>:<max>:<stepSec>"
//
// For now we stow the profile string in the node’s Metadata
// and set CarbonIntensity to the “mean” value; the CIScheduler
// wrapper can look at Metadata to fetch a dynamic CI per tick.
func LoadNodesFromCSV(path string) []*core.SimulatedNode {
    f, err := os.Open(path)
    if err != nil {
        log.Fatalf("LoadNodesFromCSV: open %s: %v", path, err)
    }
    defer f.Close()

    r := csv.NewReader(f)
    // read header
    if _, err := r.Read(); err != nil {
        log.Fatalf("LoadNodesFromCSV: read header: %v", err)
    }

    var nodes []*core.SimulatedNode
    for {
        rec, err := r.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            log.Fatalf("LoadNodesFromCSV: read record: %v", err)
        }

        name := rec[0]
        cpu, _ := strconv.Atoi(rec[1])
        mem, _ := strconv.Atoi(rec[2])
        profile := rec[3]

        // default baseCI = 0
        var baseCI float64
        parts := strings.Split(profile, ":")
        switch parts[0] {
        case "static":
            baseCI, _ = strconv.ParseFloat(parts[1], 64)
        case "sine":
            // sine:<mean>:<amp>:<periodSec>
            mean, _ := strconv.ParseFloat(parts[1], 64)
            baseCI = mean
        case "randwalk":
            // randwalk:<min>:<max>:<stepSec>
            minv, _ := strconv.ParseFloat(parts[1], 64)
            maxv, _ := strconv.ParseFloat(parts[2], 64)
            baseCI = (minv + maxv) / 2.0
        default:
            log.Printf("LoadNodesFromCSV: unknown ci_profile %q, defaulting to 0", profile)
        }

        node := core.NewNode(name, float64(cpu), float64(mem), baseCI)
        // stash the profile string for my CI‐aware wrapper:
        node.Metadata = map[string]string{"ci_profile": profile}
        nodes = append(nodes, node)
    }
    return nodes
}

// LoadWorkloadsFromCSV parses a CSV of:
//
//    id,submit,cpu,mem,duration,tag
//
// and returns a slice of Workload with SubmitTime, Duration,
// CPU, Memory and Tag populated.
func LoadWorkloadsFromCSV(path string) []core.Workload {
    f, err := os.Open(path)
    if err != nil {
        log.Fatalf("LoadWorkloadsFromCSV: open %s: %v", path, err)
    }
    defer f.Close()

    r := csv.NewReader(f)
    // header
    if _, err := r.Read(); err != nil {
        log.Fatalf("LoadWorkloadsFromCSV: read header: %v", err)
    }

    var wls []core.Workload
    for {
        rec, err := r.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            log.Fatalf("LoadWorkloadsFromCSV: read record: %v", err)
        }
        id := rec[0]
        submit, _ := time.Parse(time.RFC3339, rec[1])
        cpuF, _  := strconv.ParseFloat(rec[2], 64)
        memF, _  := strconv.ParseFloat(rec[3], 64)
        durSec, _:= strconv.Atoi(rec[4])
        tag      := ""
        if len(rec) >= 6 {
            tag = rec[5]
        }

        wls = append(wls, core.Workload{
            ID:         id,
            SubmitTime: submit,
            Duration:   time.Duration(durSec) * time.Second,
            CPU:        cpuF,
            Memory:     memF,
            Tag:        tag,
        })
    }
    return wls
}
