package reporter

import (
	"bytes"
	"encoding/csv"
	"testing"
)

func TestCSVReporter(t *testing.T) {
	var buf bytes.Buffer
	if err := (CSV{}).Report(&buf, sampleResults()); err != nil {
		t.Fatal(err)
	}

	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	// Header + 2 issues; the failed target contributes no rows.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3:\n%v", len(rows), rows)
	}
	if rows[0][0] != "target" || rows[0][2] != "code" {
		t.Errorf("header = %v", rows[0])
	}
	if rows[1][0] != "https://example.com" || rows[1][1] != "error" || rows[1][2] != "image-alt" {
		t.Errorf("first row = %v", rows[1])
	}
}
