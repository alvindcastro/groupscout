package storage

import (
	"context"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name    string
		a, b    []float32
		wantMin float32
		wantMax float32
	}{
		{
			name:    "identical vectors",
			a:       []float32{1, 0, 0},
			b:       []float32{1, 0, 0},
			wantMin: 0.999,
			wantMax: 1.001,
		},
		{
			name:    "orthogonal vectors",
			a:       []float32{1, 0, 0},
			b:       []float32{0, 1, 0},
			wantMin: -0.001,
			wantMax: 0.001,
		},
		{
			name:    "opposite vectors",
			a:       []float32{1, 0, 0},
			b:       []float32{-1, 0, 0},
			wantMin: -1.001,
			wantMax: -0.999,
		},
		{
			name:    "zero vector returns 0",
			a:       []float32{0, 0, 0},
			b:       []float32{1, 0, 0},
			wantMin: -0.001,
			wantMax: 0.001,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("cosineSimilarity = %f, want [%f, %f]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestInMemoryEmbeddingStore_Similar(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	Migrate(db, ":memory:")

	store := NewEmbeddingStore(db, ":memory:")
	ctx := context.Background()

	// Seed three leads with known embeddings
	// vec1 and vec2 are similar; vec3 is different
	vec1 := []float32{1.0, 0.0, 0.0}
	vec2 := []float32{0.9, 0.1, 0.0}
	vec3 := []float32{0.0, 0.0, 1.0}

	store.Save(ctx, "lead-1", "test-model", vec1)
	store.Save(ctx, "lead-2", "test-model", vec2)
	store.Save(ctx, "lead-3", "test-model", vec3)

	// Query similar to vec1 — should return lead-1 and lead-2 first
	ids, err := store.Similar(ctx, vec1, 2)
	if err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("Similar returned %d results, want 2", len(ids))
	}
	if ids[0] != "lead-1" {
		t.Errorf("top result = %q, want lead-1", ids[0])
	}
	if ids[1] != "lead-2" {
		t.Errorf("second result = %q, want lead-2", ids[1])
	}
}
