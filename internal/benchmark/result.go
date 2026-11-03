package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
)

const benchmarkFileSize = "1Gi"

type workloadResult struct {
	Name      string
	IOType    string
	IOPS      float64
	Bandwidth float64 // MiB/s
	LatencyMS float64
	Success   bool
	Error     string
}

type fioOutput struct {
	Jobs []struct {
		Read  fioDirection \`json:"read"\`
		Write fioDirection \`json:"write"\`
	} \`json:"jobs"\`
}

type fioDirection struct {
	IOPS    float64 `json:"iops"`
	BW      float64 `json:"bw"`      // KiB/s in FIO JSON
	BWBytes float64 `json:"bw_bytes"` // bytes/s when available
	CLAT    fioLatency `json:"clat"`
	CLATNS  fioLatency `json:"clat_ns"`
	LAT     fioLatency `json:"lat"`
	LATNS   fioLatency `json:"lat_ns"`
}

type fioLatency struct {
	Mean float64 `json:"mean"`
	Unit string  `json:"unit"`
}

func workloadsForProfile(profile string) []string {
	switch profile {
	case "read":
		return []string{"seq-read", "rand-read"}
	case "write":
		return []string{"seq-write", "rand-write"}
	case "all":
		return []string{"seq-read", "seq-write", "rand-read", "rand-write"}
	default:
		return nil
	}
}

func parseWorkloadResults(logs, profile string) []workloadResult {
	workloads := workloadsForProfile(profile)
	results := make([]workloadResult, 0, len(workloads))

	for _, name := range workloads {
		result := workloadResult{Name: name, IOType: "Sequential"}
		if strings.HasPrefix(name, "rand-") {
			result.IOType = "Random"
		}

		beginMarker := "PVBench:RESULT:" + name + ":BEGIN"
		endMarker := "PVBench:RESULT:" + name + ":END"
		begin := strings.Index(logs, beginMarker)
		if begin < 0 {
			result.Error = "workload did not start"
			results = append(results, result)
			continue
		}
		begin += len(beginMarker)
		end := strings.Index(logs[begin:], endMarker)
		if end < 0 {
			result.Error = "workload did not complete or FIO output is incomplete"
			results = append(results, result)
			continue
		}

		payload := strings.TrimSpace(logs[begin : begin+end])
		var output fioOutput
		if err := json.Unmarshal([]byte(payload), &output); err != nil {
			result.Error = "invalid FIO JSON: " + err.Error()
			results = append(results, result)
			continue
		}
		if len(output.Jobs) == 0 {
			result.Error = "FIO JSON contains no jobs"
			results = append(results, result)
			continue
		}

		direction := output.Jobs[0].Read
		if strings.HasSuffix(name, "write") {
			direction = output.Jobs[0].Write
		}
		result.IOPS = direction.IOPS
		if direction.BWBytes > 0 {
			result.Bandwidth = direction.BWBytes / (1024 * 1024)
		} else {
			result.Bandwidth = direction.BW / 1024
		}
		result.LatencyMS = latencyMilliseconds(direction)
		if result.IOPS <= 0 || result.Bandwidth <= 0 || result.LatencyMS < 0 {
			result.Error = "FIO JSON is missing valid IOPS, bandwidth, or latency metrics"
			results = append(results, result)
			continue
		}
		result.Success = true
		results = append(results, result)
	}

	return results
}

func latencyMilliseconds(direction fioDirection) float64 {
	if direction.CLATNS.Mean > 0 {
		return direction.CLATNS.Mean / 1_000_000
	}
	if direction.CLAT.Mean > 0 {
		switch strings.ToLower(direction.CLAT.Unit) {
		case "nsec", "ns":
			return direction.CLAT.Mean / 1_000_000
		case "msec", "ms":
			return direction.CLAT.Mean
		default:
			return direction.CLAT.Mean / 1000 // FIO 3.x reports clat in usec.
		}
	}
	if direction.LATNS.Mean > 0 {
		return direction.LATNS.Mean / 1_000_000
	}
	if direction.LAT.Mean > 0 {
		switch strings.ToLower(direction.LAT.Unit) {
		case "nsec", "ns":
			return direction.LAT.Mean / 1_000_000
		case "msec", "ms":
			return direction.LAT.Mean
		default:
			return direction.LAT.Mean / 1000
		}
	}
	return -1
}

func renderSummary(output io.Writer, namespace, storageClass, size, profile string, results []workloadResult, executionErr error) {
	fmt.Fprintln(output, "PVBench - Storage Benchmark")
	fmt.Fprintln(output, "===========================")
	fmt.Fprintln(output, "\nStorage Configuration")
	fmt.Fprintln(output, "---------------------")
	fmt.Fprintf(output, "Namespace:\t%s\n", namespace)
	fmt.Fprintf(output, "StorageClass:\t%s\n", storageClass)
	fmt.Fprintf(output, "PVC Size:\t%s\n", size)
	fmt.Fprintf(output, "Profile:\t%s\n", profile)
	fmt.Fprintf(output, "Benchmark File:\t%s\n", benchmarkFileSize)

	fmt.Fprintln(output, "\nBenchmark Results")
	fmt.Fprintln(output, "-----------------")
	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "WORKLOAD\tI/O TYPE\tIOPS\tBANDWIDTH\tAVG LATENCY\tRESULT")
	for _, result := range results {
		if !result.Success {
			fmt.Fprintf(table, "%s\t%s\t-\t-\t-\tFAILED\n", result.Name, result.IOType)
			continue
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%.2f MiB/s\t%.2f ms\tSUCCESS\n",
			result.Name, result.IOType, formatNumber(result.IOPS), result.Bandwidth, result.LatencyMS)
	}
	_ = table.Flush()

	completed := 0
	for _, result := range results {
		if result.Success {
			completed++
		}
	}
	status := "SUCCESS"
	if completed != len(results) || executionErr != nil {
		status = "FAILED"
	}
	fmt.Fprintln(output, "\nSummary")
	fmt.Fprintln(output, "-------")
	fmt.Fprintf(output, "Workloads completed:\t%d/%d\n", completed, len(results))
	fmt.Fprintf(output, "Status:\t%s\n", status)

	for _, result := range results {
		if result.Error != "" {
			fmt.Fprintf(output, "  %s: %s\n", result.Name, result.Error)
		}
	}
	if executionErr != nil {
		fmt.Fprintf(output, "  Error: %s\n", executionErr)
	}
}

func formatNumber(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 0, 64)
	if len(formatted) <= 3 {
		return formatted
	}
	var b strings.Builder
	first := len(formatted) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(formatted[:first])
	for i := first; i < len(formatted); i += 3 {
		b.WriteByte(',')
		b.WriteString(formatted[i : i+3])
	}
	return b.String()
}
