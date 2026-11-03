package benchmark

import (
	"strings"
	"testing"
)

func TestParseWorkloadResults(t *testing.T) {
	logs := `PVBench:RESULT:seq-read:BEGIN
{"jobs":[{"read":{"iops":1234.5,"bw_bytes":1293942784,"clat_ns":{"mean":810000}},"write":{}}]}
PVBench:RESULT:seq-read:END
PVBench:RESULT:rand-read:BEGIN
{"jobs":[{"read":{"iops":8500,"bw":34000,"clat":{"mean":1880,"unit":"usec"}},"write":{}}]}
PVBench:RESULT:rand-read:END
`

	results := parseWorkloadResults(logs, "read")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("seq-read failed: %s", results[0].Error)
	}
	if results[0].IOPS != 1234.5 {
		t.Errorf("unexpected seq-read IOPS: %v", results[0].IOPS)
	}
	if results[0].Bandwidth < 1233 || results[0].Bandwidth > 1235 {
		t.Errorf("unexpected seq-read bandwidth: %v MiB/s", results[0].Bandwidth)
	}
	if results[0].LatencyMS != 0.81 {
		t.Errorf("unexpected seq-read latency: %v ms", results[0].LatencyMS)
	}
	if !results[1].Success {
		t.Fatalf("rand-read failed: %s", results[1].Error)
	}
	if results[1].LatencyMS != 1.88 {
		t.Errorf("unexpected rand-read latency: %v ms", results[1].LatencyMS)
	}
}

func TestParseWorkloadResultsReportsMissingWorkload(t *testing.T) {
	results := parseWorkloadResults("PVBench:RESULT:seq-read:BEGIN\nnot-json", "read")
	if len(results) != 2 {
		t.Fatalf("expected 2 workload rows, got %d", len(results))
	}
	if results[0].Success || !strings.Contains(results[0].Error, "incomplete") {
		t.Errorf("expected incomplete seq-read failure, got %+v", results[0])
	}
	if results[1].Success || !strings.Contains(results[1].Error, "did not start") {
		t.Errorf("expected missing rand-read failure, got %+v", results[1])
	}
}
