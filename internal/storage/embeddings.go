package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/pgvector/pgvector-go"
)

// EmbeddingStore handles storage and similarity search for lead embeddings.
type EmbeddingStore interface {
	Save(ctx context.Context, leadID, model string, embedding []float32) error
	Similar(ctx context.Context, embedding []float32, limit int) ([]string, error)
}

// NewEmbeddingStore returns a driver-specific EmbeddingStore.
func NewEmbeddingStore(db *sql.DB, dsn string) EmbeddingStore {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return &postgresEmbeddingStore{db: db}
	}
	return &inMemoryEmbeddingStore{db: db}
}

// postgresEmbeddingStore uses pgvector for similarity search in PostgreSQL.
type postgresEmbeddingStore struct {
	db *sql.DB
}

func (s *postgresEmbeddingStore) Save(ctx context.Context, leadID, model string, embedding []float32) error {
	query := `
		INSERT INTO lead_embeddings (lead_id, model, embedding)
		VALUES ($1, $2, $3)
		ON CONFLICT (lead_id) DO UPDATE SET
			model = EXCLUDED.model,
			embedding = EXCLUDED.embedding,
			created_at = NOW()`
	_, err := s.db.ExecContext(ctx, query, leadID, model, pgvector.NewVector(embedding))
	return err
}

func (s *postgresEmbeddingStore) Similar(ctx context.Context, embedding []float32, limit int) ([]string, error) {
	query := `
		SELECT lead_id FROM lead_embeddings
		ORDER BY embedding <=> $1
		LIMIT $2`
	rows, err := s.db.QueryContext(ctx, query, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// inMemoryEmbeddingStore uses a SQLite table and computes cosine similarity in Go.
type inMemoryEmbeddingStore struct {
	db *sql.DB
}

func (s *inMemoryEmbeddingStore) Save(ctx context.Context, leadID, model string, embedding []float32) error {
	embJSON, err := json.Marshal(embedding)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO lead_embeddings_sqlite (lead_id, model, embedding, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(lead_id) DO UPDATE SET
			model = excluded.model,
			embedding = excluded.embedding,
			created_at = CURRENT_TIMESTAMP`
	_, err = s.db.ExecContext(ctx, query, leadID, model, string(embJSON))
	return err
}

func (s *inMemoryEmbeddingStore) Similar(ctx context.Context, embedding []float32, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT lead_id, embedding FROM lead_embeddings_sqlite")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type score struct {
		id    string
		value float32
	}
	var scores []score

	for rows.Next() {
		var id, embStr string
		if err := rows.Scan(&id, &embStr); err != nil {
			return nil, err
		}

		var emb []float32
		if err := json.Unmarshal([]byte(embStr), &emb); err != nil {
			continue
		}

		scores = append(scores, score{
			id:    id,
			value: cosineSimilarity(embedding, emb),
		})
	}

	// Sort by similarity descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].value > scores[j].value
	})

	if len(scores) > limit {
		scores = scores[:limit]
	}

	var ids []string
	for _, sc := range scores {
		ids = append(ids, sc.id)
	}
	return ids, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func (s *inMemoryEmbeddingStore) Rebind(query string) string {
	// inMemoryEmbeddingStore uses SQLite (?, ?, ?) which doesn't need rebinding
	return query
}
