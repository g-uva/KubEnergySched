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
	"regexp"
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

		// Example: scaph_process_memory_bytes{exe="/bin/sh",pid="123"} 4567
		metricParts := strings.Fields(line)
		if len(metricParts) < 2 {
			continue
		}

		metricFull := metricParts[0]       // e.g., scaph_process_memory_bytes{...}
		metricValue := metricParts[1]      // e.g., 4567

		metricName, labelSet := extractMetricKey(metricFull)
		if metricName == "" {
			continue
		}

		key := fmt.Sprintf("%s__%s", metricName, labelSet)
		metricMap[key] = metricValue
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

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Printf("Failed to open CSV: %v", err)
		http.Error(w, "CSV open error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Load or initialize the CSV
	reader := csv.NewReader(f)
	existingHeader := []string{}
	if fileExists {
		headerLine, err := reader.Read()
		if err == nil {
			existingHeader = headerLine
		}
	}

	// Collect all unique keys from existing file and current batch
	columnSet := map[string]bool{}
	for _, key := range existingHeader[1:] {
		columnSet[key] = true
	}
	for key := range metricMap {
		columnSet[key] = true
	}

	// Determine final column order
	columnOrder := []string{}
	for col := range columnSet {
		columnOrder = append(columnOrder, col)
	}
	sort.Strings(columnOrder)

	// Rewrite the file with the new header if needed
	var updatedRows [][]string
	if fileExists {
		_, err := f.Seek(0, 0)
		if err != nil {
			log.Printf("Failed to rewind file: %v", err)
			http.Error(w, "Seek error", http.StatusInternalServerError)
			return
		}
		existingRows, err := reader.ReadAll()
		if err == nil && len(existingRows) > 0 {
			updatedRows = append(updatedRows, []string{"timestamp"})
			updatedRows[0] = append(updatedRows[0], columnOrder...)
			for _, row := range existingRows[1:] {
				rowMap := make(map[string]string)
				for i := 1; i < len(existingHeader) && i < len(row); i++ {
					rowMap[existingHeader[i]] = row[i]
				}
				newRow := []string{row[0]}
				for _, col := range columnOrder {
					newRow = append(newRow, rowMap[col])
				}
				updatedRows = append(updatedRows, newRow)
			}
		}
	}

	// Add the new row
	newRow := []string{timestamp}
	for _, col := range columnOrder {
		newRow = append(newRow, metricMap[col])
	}
	updatedRows = append(updatedRows, newRow)

	// Rewrite the file
	f.Truncate(0)
	f.Seek(0, 0)
	writer := csv.NewWriter(f)
	defer writer.Flush()
	for _, row := range updatedRows {
		writer.Write(row)
	}

	log.Printf("[CentralUnit] Saved metrics at %s", timestamp)
	w.WriteHeader(http.StatusOK)
}

// extractMetricKey parses the metric name and returns a flat key
func extractMetricKey(metric string) (string, string) {
	nameEnd := strings.Index(metric, "{")
	if nameEnd == -1 {
		return metric, "unknown_unknown"
	}
	metricName := metric[:nameEnd]
	labelSection := metric[nameEnd+1 : len(metric)-1]
	labels := parseLabels(labelSection)
	exe := strings.ReplaceAll(labels["exe"], ",", "")
	pid := labels["pid"]
	if exe == "" || pid == "" {
		return metricName, "unknown_unknown"
	}
	return metricName, fmt.Sprintf("%s__%s", exe, pid)
}

// parseLabels parses key="value",key2="value2" into a map
func parseLabels(labelStr string) map[string]string {
	result := make(map[string]string)
	matches := regexp.MustCompile(`(\w+)="(.*?)"`).FindAllStringSubmatch(labelStr, -1)
	for _, match := range matches {
		if len(match) == 3 {
			result[match[1]] = match[2]
		}
	}
	return result
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