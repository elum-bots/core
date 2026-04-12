package bot

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreCRUD(t *testing.T) {
	s := NewMemoryStore()
	if got, err := s.Get(context.Background(), "k"); err != nil || got != nil {
		t.Fatalf("unexpected get: %v %#v", err, got)
	}
	if err := s.Set(context.Background(), "k", []byte("v"), time.Second); err != nil {
		t.Fatalf("set err: %v", err)
	}
	got, err := s.Get(context.Background(), "k")
	if err != nil || string(got) != "v" {
		t.Fatalf("get mismatch: %v %q", err, string(got))
	}
	if err := s.Delete(context.Background(), "k"); err != nil {
		t.Fatalf("delete err: %v", err)
	}
	got, err = s.Get(context.Background(), "k")
	if err != nil || got != nil {
		t.Fatalf("expected nil after delete: %v %#v", err, got)
	}
}
