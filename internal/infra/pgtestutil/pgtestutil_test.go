package pgtestutil

import (
	"strings"
	"testing"
)

func TestReplaceDBInDSN(t *testing.T) {
	in := "postgres://myuser:mypassword@localhost:5432/postgres?sslmode=disable"
	out, err := ReplaceDBInDSN(in, "testdb_foo")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("OUT:", out) // should contain testdb_foo
	if !strings.Contains(out, "testdb_foo") {
		t.Fatalf("db not replaced: %s", out)
	}
}
