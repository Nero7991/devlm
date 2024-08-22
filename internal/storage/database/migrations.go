package database

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
)

type MigrationFunc func(ctx context.Context, tx pgx.Tx) error

type Migration struct {
	Version int64
	Up      MigrationFunc
	Down    MigrationFunc
}

var (
	Migrations = []Migration{
		{
			Version: 1,
			Up: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					CREATE TABLE IF NOT EXISTS projects (
						id SERIAL PRIMARY KEY,
						name TEXT NOT NULL,
						description TEXT,
						created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
						deleted_at TIMESTAMP WITH TIME ZONE
					);

					CREATE TABLE IF NOT EXISTS tasks (
						id SERIAL PRIMARY KEY,
						project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
						description TEXT NOT NULL,
						status TEXT NOT NULL,
						created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
						deleted_at TIMESTAMP WITH TIME ZONE
					);

					CREATE TABLE IF NOT EXISTS code_executions (
						id SERIAL PRIMARY KEY,
						task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
						code TEXT NOT NULL,
						result TEXT,
						created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
					);

					CREATE TABLE IF NOT EXISTS llm_requests (
						id SERIAL PRIMARY KEY,
						task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
						prompt TEXT NOT NULL,
						response TEXT,
						created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
					);

					CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
					CREATE INDEX IF NOT EXISTS idx_code_executions_task_id ON code_executions(task_id);
					CREATE INDEX IF NOT EXISTS idx_llm_requests_task_id ON llm_requests(task_id);
				`)
				return err
			},
			Down: func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					DROP TABLE IF EXISTS llm_requests;
					DROP TABLE IF EXISTS code_executions;
					DROP TABLE IF EXISTS tasks;
					DROP TABLE IF EXISTS projects;
				`)
				return err
			},
		},
	}

	migrationCache     = make(map[int64]Migration)
	migrationCacheLock sync.RWMutex
)

func init() {
	for _, migration := range Migrations {
		migrationCache[migration.Version] = migration
	}
}

func RunMigrations(db *PostgresDB) error {
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS migrations (
			version INT8 PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	currentVersion, err := GetCurrentMigrationVersion(db)
	if err != nil {
		return err
	}

	sort.Slice(Migrations, func(i, j int) bool {
		return Migrations[i].Version < Migrations[j].Version
	})

	for _, migration := range Migrations {
		if migration.Version > currentVersion {
			err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
				if err := migration.Up(ctx, tx); err != nil {
					return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
				}

				_, err := tx.Exec(ctx, "INSERT INTO migrations (version) VALUES ($1)", migration.Version)
				if err != nil {
					return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
				}

				return nil
			})

			if err != nil {
				return err
			}

			log.Printf("Applied migration %d", migration.Version)
		}
	}

	return nil
}

func RollbackMigration(db *PostgresDB) error {
	ctx := context.Background()

	lastVersion, err := GetCurrentMigrationVersion(db)
	if err != nil {
		return err
	}

	if lastVersion == 0 {
		return fmt.Errorf("no migrations to roll back")
	}

	migrationCacheLock.RLock()
	migration, exists := migrationCache[lastVersion]
	migrationCacheLock.RUnlock()

	if !exists {
		return fmt.Errorf("migration %d not found", lastVersion)
	}

	err = db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		if err := migration.Down(ctx, tx); err != nil {
			return fmt.Errorf("failed to roll back migration %d: %w", lastVersion, err)
		}

		_, err := tx.Exec(ctx, "DELETE FROM migrations WHERE version = $1", lastVersion)
		if err != nil {
			return fmt.Errorf("failed to remove migration record %d: %w", lastVersion, err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Printf("Rolled back migration %d", lastVersion)
	return nil
}

func RunMigrationsUpTo(db *PostgresDB, targetVersion int64) error {
	ctx := context.Background()

	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS migrations (
			version INT8 PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	currentVersion, err := GetCurrentMigrationVersion(db)
	if err != nil {
		return err
	}

	if targetVersion < currentVersion {
		return fmt.Errorf("target version %d is lower than current version %d", targetVersion, currentVersion)
	}

	sort.Slice(Migrations, func(i, j int) bool {
		return Migrations[i].Version < Migrations[j].Version
	})

	for _, migration := range Migrations {
		if migration.Version > currentVersion && migration.Version <= targetVersion {
			err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
				if err := migration.Up(ctx, tx); err != nil {
					return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
				}

				_, err := tx.Exec(ctx, "INSERT INTO migrations (version) VALUES ($1)", migration.Version)
				if err != nil {
					return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
				}

				return nil
			})

			if err != nil {
				return err
			}

			log.Printf("Applied migration %d", migration.Version)
		}
	}

	return nil
}

func RollbackMigrationsTo(db *PostgresDB, targetVersion int64) error {
	ctx := context.Background()

	lastVersion, err := GetCurrentMigrationVersion(db)
	if err != nil {
		return err
	}

	if lastVersion == 0 {
		return fmt.Errorf("no migrations to roll back")
	}

	if targetVersion >= lastVersion {
		return fmt.Errorf("target version %d is greater than or equal to current version %d", targetVersion, lastVersion)
	}

	sort.Slice(Migrations, func(i, j int) bool {
		return Migrations[i].Version > Migrations[j].Version
	})

	for _, migration := range Migrations {
		if migration.Version > targetVersion && migration.Version <= lastVersion {
			err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
				if err := migration.Down(ctx, tx); err != nil {
					return fmt.Errorf("failed to roll back migration %d: %w", migration.Version, err)
				}

				_, err := tx.Exec(ctx, "DELETE FROM migrations WHERE version = $1", migration.Version)
				if err != nil {
					return fmt.Errorf("failed to remove migration record %d: %w", migration.Version, err)
				}

				return nil
			})

			if err != nil {
				return err
			}

			log.Printf("Rolled back migration %d", migration.Version)
		}
	}

	return nil
}

func GetCurrentMigrationVersion(db *PostgresDB) (int64, error) {
	ctx := context.Background()
	var currentVersion int64
	err := db.pool.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&currentVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to get current migration version: %w", err)
	}
	return currentVersion, nil
}

func ListAppliedMigrations(db *PostgresDB) ([]int64, error) {
	ctx := context.Background()
	rows, err := db.pool.Query(ctx, "SELECT version FROM migrations ORDER BY version ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var appliedMigrations []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		appliedMigrations = append(appliedMigrations, version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over migration rows: %w", err)
	}

	return appliedMigrations, nil
}

func GetMigrationDetails(db *PostgresDB) ([]MigrationDetail, error) {
	ctx := context.Background()
	rows, err := db.pool.Query(ctx, "SELECT version, applied_at FROM migrations ORDER BY version ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query migration details: %w", err)
	}
	defer rows.Close()

	var details []MigrationDetail
	for rows.Next() {
		var detail MigrationDetail
		if err := rows.Scan(&detail.Version, &detail.AppliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration detail: %w", err)
		}
		details = append(details, detail)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over migration detail rows: %w", err)
	}

	return details, nil
}

type MigrationDetail struct {
	Version   int64
	AppliedAt time.Time
}