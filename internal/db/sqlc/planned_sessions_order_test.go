package sqlc_test

import (
	"os"
	"strings"
	"testing"
)

func TestListPlannedSessionsByRangeOrdersStableByDateAndID(t *testing.T) {
	for _, file := range []string{"../queries/planned_sessions.sql", "planned_sessions.sql.go"} {
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(contents)
		marker := "ListPlannedSessionsByRange"
		start := strings.Index(text, marker)
		if start < 0 {
			t.Fatalf("%s does not contain %s", file, marker)
		}
		block := text[start:]
		if end := strings.Index(block, "-- name: "); end >= 0 {
			block = block[:end]
		}
		if !strings.Contains(block, "ORDER BY scheduled_on ASC, id ASC") {
			t.Errorf("%s list query must order by scheduled_on ASC, id ASC", file)
		}
	}
}
