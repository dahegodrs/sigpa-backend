package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/alcaldia/sigpa-backend/internal/config"
)

// NewConnection crea y valida la conexión a PostgreSQL (Supabase).
//
// Usa el driver pgx en modo "database/sql" (stdlib) para mantener el resto
// del código (repositorios que usan *sql.DB) sin cambios estructurales.
//
// La cadena de conexión se construye a partir de las variables DB_HOST,
// DB_PORT, DB_USER, DB_PASSWORD, DB_NAME. Para Supabase, normalmente:
//
//	DB_HOST=aws-0-xx.pooler.supabase.com (o db.xxxx.supabase.co)
//	DB_PORT=5432 (o 6543 si usas el connection pooler)
//	DB_USER=postgres.xxxxxxxxxxxx (o postgres)
//	DB_NAME=postgres
func NewConnection(cfg *config.Config) (*sql.DB, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=require",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión con PostgreSQL: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error al hacer ping a PostgreSQL: %w", err)
	}

	log.Println("Conexión a PostgreSQL (Supabase) establecida correctamente")
	return db, nil
}
