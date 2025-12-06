package snow

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// SQLComputer implements SnowfallComputer using PostgreSQL functions.
// This strategy delegates computation to SQL functions in the database.
type SQLComputer struct {
	db           *sql.DB
	logger       *zap.SugaredLogger
	stationName  string
	baseDistance float64
}

// NewSQLComputer creates a SQL-based snowfall computer
func NewSQLComputer(db *sql.DB, logger *zap.SugaredLogger, station string, baseDistance float64) *SQLComputer {
	return &SQLComputer{
		db:           db,
		logger:       logger,
		stationName:  station,
		baseDistance: baseDistance,
	}
}

// Compute24h calls the SQL function for 24-hour snow calculation
func (s *SQLComputer) Compute24h(ctx context.Context) (float64, error) {
	var snowMM sql.NullFloat64
	query := `SELECT get_new_snow_24h($1, $2)`
	err := s.db.QueryRowContext(ctx, query, s.stationName, s.baseDistance).Scan(&snowMM)
	if err != nil {
		return 0, err
	}
	if !snowMM.Valid {
		return 0, nil
	}
	return snowMM.Float64, nil
}

// Compute72h calls the SQL function for 72-hour snow calculation
func (s *SQLComputer) Compute72h(ctx context.Context) (float64, error) {
	var snowMM sql.NullFloat64
	query := `SELECT get_new_snow_72h($1, $2)`
	err := s.db.QueryRowContext(ctx, query, s.stationName, s.baseDistance).Scan(&snowMM)
	if err != nil {
		return 0, err
	}
	if !snowMM.Valid {
		return 0, nil
	}
	return snowMM.Float64, nil
}

// ComputeSeasonal calls the SQL function for seasonal snow calculation
func (s *SQLComputer) ComputeSeasonal(ctx context.Context) (float64, error) {
	var snowMM sql.NullFloat64
	query := `SELECT get_new_snow_seasonal($1, $2)`
	err := s.db.QueryRowContext(ctx, query, s.stationName, s.baseDistance).Scan(&snowMM)
	if err != nil {
		return 0, err
	}
	if !snowMM.Valid {
		return 0, nil
	}
	return snowMM.Float64, nil
}
