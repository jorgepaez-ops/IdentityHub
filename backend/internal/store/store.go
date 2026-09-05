// Package store encapsula el acceso a PostgreSQL.
//
// A partir de la semana 2 las consultas se generan con sqlc desde
// db/migrations y db/queries. Este archivo solo mantiene el pool y las sondas.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

// New abre el pool y verifica la conexión antes de devolver. Fallar aquí es
// preferible a descubrir la base de datos caída en la primera petición real.
func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DSN de PostgreSQL inválido: %w", err)
	}

	// Argon2id reserva 64 MiB por verificación (ver adr/0004), así que la
	// concurrencia útil de la API está acotada por memoria, no por la base de
	// datos. Un pool grande solo serviría para acumular trabajo en cola.
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear el pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("la base de datos no responde: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Ping alimenta la sonda /readyz.
func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(ctx)
}

func (s *Store) Close() { s.pool.Close() }
