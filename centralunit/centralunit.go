package main

import (
	"encoding/json"
	"net/http"
	"fmt"
	"log"
	"os"
	"kube-scheduler/pkg/core"
)

var unit core.CentralUnit

func handleWorkloadIngest(w http.ResponseWriter, r *http.Request) {
	var job core.Workload
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "invalid job format", http.StatusBadRequest)
		return
	}
	go unit.Dispatch([]core.Workload{job})
	w.WriteHeader(http.StatusOK)
	fmt.Printf("[CentralUnit] Ingested job: %s\n", job.ID)
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
		Strategy: &core.RoundRobin{},
	}

	http.HandleFunc("/ingest", handleWorkloadIngest)
	port := "8080"
	if p := os.Getenv("CENTRAL_PORT"); p != "" {
		port = p
	}
	fmt.Printf("CentralUnit API listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}