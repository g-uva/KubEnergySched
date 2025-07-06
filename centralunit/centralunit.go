package main

import (
	"encoding/json"
	"net/http"
	"fmt"
	"log"
	"os"
	"kube-scheduler/pkg/core"
	"time"
	"io"
	"encoding/csv"
	"strings"
	"sort"
)

var unit core.CentralUnit

func handleWorkloadIngest(w http.ResponseWriter, r *http.Request) {
	var job core.Workload
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "invalid job format", http.StatusBadRequest)
		return
	}

	go unit.DispatchAll([]core.Workload{job})
	w.WriteHeader(http.StatusOK)
	fmt.Printf("[CentralUnit] Ingested job: %s\n", job.ID)

	time.Sleep(3 * time.Second)
	core.PrintDecisionTable()
}

func handleMetricsIngest(w http.ResponseWriter, r *http.Request) {
	log.Println("[CentralUnit] /metrics-ingest hit")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body.", http.StatusInternalServerError)
		return
	}

	lines := strings.Split(string(data), "\n")
	timestamp := time.Now().Format(time.RFC3339)

	metricMap := make(map[string]string)

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" || !strings.HasPrefix(line, "scaph_") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		metricMap[parts[0]] = parts[1]
	}

	if len(metricMap) == 0 {
		log.Println("[CentralUnit] No scaph_ metrics found, skipping.")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	filePath := "/data/metrics_aggregated.csv"
	fileExists := false
	if _, err := os.Stat(filePath); err == nil {
		fileExists = true
	}

	var file *os.File
	if fileExists {
		file, err = os.OpenFile(filePath, os.O_RDWR, 0644)
	} else {
		file, err = os.Create(filePath)
	}
	if err != nil {
		log.Printf("Failed to open CSV: %v", err)
		http.Error(w, "CSV open error", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	var headers []string
	existingRows := [][]string{}

	if fileExists {
		file.Seek(0, 0)
		existingRows, _ = reader.ReadAll()
		if len(existingRows) > 0 {
			headers = existingRows[0][1:] // remove "timestamp"
		}
	}

	// Union: existing headers + new ones (in order)
	headerSet := map[string]bool{}
	for _, h := range headers {
		headerSet[h] = true
	}
	for key := range metricMap {
		if !headerSet[key] {
			headers = append(headers, key)
			headerSet[key] = true
		}
	}
	sort.Strings(headers)

	// Rewrite file
	file.Truncate(0)
	file.Seek(0, 0)
	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write(append([]string{"timestamp"}, headers...))

	for i := 1; i < len(existingRows); i++ {
		rowMap := make(map[string]string)
		for j := 1; j < len(existingRows[i]); j++ {
			if j < len(existingRows[0]) {
				rowMap[existingRows[0][j]] = existingRows[i][j]
			}
		}
		row := []string{existingRows[i][0]}
		for _, h := range headers {
			row = append(row, rowMap[h])
		}
		writer.Write(row)
	}

	newRow := []string{timestamp}
	for _, h := range headers {
		newRow = append(newRow, metricMap[h])
	}
	writer.Write(newRow)

	log.Printf("[CentralUnit] Saved metrics at %s", timestamp)
	w.WriteHeader(http.StatusOK)
}

func main() {
	fmt.Println("Loading clusters from file...")
	clustersRaw, err := core.LoadClustersFromFile("/config/clusters.json")
	if err != nil {
		fmt.Println("Error loading clusters:", err)
		return
	}
	fmt.Printf("Loaded %d clusters from file\n", len(clustersRaw))

	var clusters []core.Cluster
	for _, rc := range clustersRaw {
		clusters = append(clusters, rc)
	}

	unit = core.CentralUnit{
		Clusters: clusters,
		// Strategy: &core.RoundRobin{},
		Strategy: &core.CIawareStrategy{},
	}

	http.HandleFunc("/ingest", handleWorkloadIngest)
	http.HandleFunc("/metrics-ingest", handleMetricsIngest)
	port := "8080"
	if p := os.Getenv("CENTRAL_PORT"); p != "" {
		port = p
	}
	fmt.Printf("CentralUnit API listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}