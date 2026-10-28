package repo

import (
	"strings"
	"testing"
	"time"

	ct "github.com/hekemen/automata/internal/domain/contact"
)

func TestBuildListWhere_SinceFiltersOnUpdatedAt(t *testing.T) {
	since := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	filters := ct.FilterOptions{ContextID: "ctx-1", Since: &since}

	where, args, argIndex, err := buildListWhere(filters)
	if err != nil {
		t.Fatalf("buildListWhere returned error: %v", err)
	}
	if !strings.Contains(where, "updated_at >=") {
		t.Errorf("where = %q, want it to contain \"updated_at >=\"", where)
	}
	if len(args) != 2 {
		t.Fatalf("args = %v (len %d), want exactly 2 (context, since)", args, len(args))
	}
	if args[0] != "ctx-1" {
		t.Errorf("args[0] = %v, want ctx-1", args[0])
	}
	if got, ok := args[1].(time.Time); !ok || !got.Equal(since) {
		t.Errorf("args[1] = %v, want %v", args[1], since)
	}
	if argIndex != 3 {
		t.Errorf("argIndex = %d, want 3 (past context and since)", argIndex)
	}
}

func TestBuildListWhere_NoSinceOmitsUpdatedAtClause(t *testing.T) {
	filters := ct.FilterOptions{ContextID: "ctx-1"}

	where, args, _, err := buildListWhere(filters)
	if err != nil {
		t.Fatalf("buildListWhere returned error: %v", err)
	}
	if strings.Contains(where, "updated_at") {
		t.Errorf("where = %q, must not mention updated_at when Since is nil", where)
	}
	if len(args) != 1 {
		t.Errorf("args = %v (len %d), want exactly 1 (context)", args, len(args))
	}
}