// Package persistence implements the writer/reader interfaces defined in
// internal/fetching, backed by PostgreSQL via pgx + sqlc.
//
// Naming follows ory/hydra: persister.go for the shared struct,
// persister_<entity>.go for entity-specific implementations.
package persistence

