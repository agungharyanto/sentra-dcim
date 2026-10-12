package repo

import "context"

// Store adapts generated queries to the service layer.
type Store struct{ q Querier }

// NewStore wraps a generated Querier.
func NewStore(db DBTX) *Store { return &Store{q: New(db)} }

// SchemaVersion returns the applied migration version and dirty flag.
func (s *Store) SchemaVersion(ctx context.Context) (int64, bool, error) {
	row, err := s.q.GetSchemaVersion(ctx)
	if err != nil {
		return 0, false, err
	}
	return row.Version, row.Dirty, nil
}
