package migrator

import "testing"

func TestLatest(t *testing.T) {
	v, err := Latest()
	if err != nil || v < 1 {
		t.Fatalf("Latest() = %d, %v", v, err)
	}
}

func TestToPgx5(t *testing.T) {
	if got := toPgx5("postgres://u:p@h:5432/d"); got != "pgx5://u:p@h:5432/d" {
		t.Fatalf("got %s", got)
	}
}
