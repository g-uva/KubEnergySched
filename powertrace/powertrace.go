package main

import (
	"bufio"
	"cloud.google.com/go/storage"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"os"
)

type PowerTrace struct {
	Time               int64   // microseconds since trace start - 5min interval
	Cell               string  // e.g. "a"
	PDU                string  // e.g. "pdu6"
	MeasuredPowerUtil  float64 // [0,1] includes cooling, etc.
	ProductionPowerUtil float64 // [0,1]
}

// DownloadAndParse fetches a gzipped power trace CSV from GCS and parses it
func DownloadAndParse(ctx context.Context, bucketName, objectName string) ([]PowerTrace, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %v", err)
	}
	defer client.Close()

	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open GCS object: %v", err)
	}
	defer rc.Close()

	gzr, err := gzip.NewReader(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip: %v", err)
	}
	defer gzr.Close()

	reader := csv.NewReader(bufio.NewReader(gzr))
	var traces []PowerTrace

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV: %v", err)
		}
		// Skip headers or malformed rows
		if len(record) < 5 || strings.ToLower(record[0]) == "time" {
			continue
		}

		timestamp, _ := strconv.ParseInt(record[0], 10, 64)
		cell := record[1]
		pdu := record[2]
		measured, _ := strconv.ParseFloat(record[3], 64)
		production, _ := strconv.ParseFloat(record[4], 64)

		traces = append(traces, PowerTrace{
			Time:                timestamp,
			Cell:                cell,
			PDU:                 pdu,
			MeasuredPowerUtil:  measured,
			ProductionPowerUtil: production,
		})
	}
	return traces, nil
}

func SaveToCSV(traces []PowerTrace, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"time", "cell", "pdu", "measured_power_util", "production_power_util"})

	for _, t := range traces {
		row := []string{
			strconv.FormatInt(t.Time, 10),
			t.Cell,
			t.PDU,
			fmt.Sprintf("%.4f", t.MeasuredPowerUtil),
			fmt.Sprintf("%.4f", t.ProductionPowerUtil),
		}
		writer.Write(row)
	}

	return nil
}

func main () {
	http.HandleFunc("/send-workloads", )
}
