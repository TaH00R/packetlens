package store

import (
	"context"
	"fmt"

	"github.com/TaH00R/packetlens/backend/internal/target"
)

func (s *SQLiteStore) ListTargets(ctx context.Context) ([]target.Config, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,type,address,enabled,interval_ms,timeout_ms FROM monitoring_targets ORDER BY type,address")
	if err != nil {
		return nil, fmt.Errorf("query targets: %w", err)
	}
	defer rows.Close()
	configs := []target.Config{}
	for rows.Next() {
		var c target.Config
		if err := rows.Scan(&c.ID, &c.Type, &c.Address, &c.Enabled, &c.IntervalMs, &c.TimeoutMs); err != nil {
			return nil, err
		}
		configs = append(configs, c)
		if len(configs) > target.MaxTargets {
			return nil, target.ErrConflict
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := target.ValidateSet(configs); err != nil {
		return nil, fmt.Errorf("invalid persisted targets: %w", err)
	}
	return configs, nil
}
func (s *SQLiteStore) ReplaceTargets(ctx context.Context, configs []target.Config) error {
	if err := target.ValidateSet(configs); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM monitoring_targets"); err != nil {
		return err
	}
	for _, c := range configs {
		if _, err := tx.ExecContext(ctx, "INSERT INTO monitoring_targets(id,type,address,enabled,interval_ms,timeout_ms) VALUES (?,?,?,?,?,?)", c.ID, c.Type, c.Address, c.Enabled, c.IntervalMs, c.TimeoutMs); err != nil {
			return fmt.Errorf("insert target: %w", err)
		}
	}
	return tx.Commit()
}
