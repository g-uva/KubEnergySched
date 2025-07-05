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

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filepath := fmt.Sprintf("/data/metrics_%s.csv", timestamp)

	err = os.WriteFile(filepath, data, 0644)
	if err != nil {
		log.Printf("Failed to write file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// log.Printf("[Central Unit] Received metrics:\n%s", string(data))
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