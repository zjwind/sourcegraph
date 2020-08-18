package sqlite

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence"
	gobserializer "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/serialization/gob"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/sqlite/batch"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/sqlite/migrate"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/sqlite/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/types"
)

var _ persistence.Writer = &sqliteDatabase{}

// assumes a database does not already exist at filename
func NewWriter(ctx context.Context, filename string) (_ persistence.Writer, err error) {
	return NewDatabase(ctx, filename, nil)
}

func (w *sqliteDatabase) WriteMeta(ctx context.Context, metaData types.MetaData) error {
	queries := []*sqlf.Query{
		sqlf.Sprintf("INSERT INTO schema_version (version) VALUES (%s)", migrate.CurrentSchemaVersion),
		sqlf.Sprintf("INSERT INTO meta (num_result_chunks) VALUES (%s)", metaData.NumResultChunks),
	}

	for _, query := range queries {
		if err := w.store.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

func (w *sqliteDatabase) WriteDocuments(ctx context.Context, documents map[string]types.DocumentData) error {
	return batch.WriteDocuments(ctx, w.store, "documents", w.serializer, documents)
}

func (w *sqliteDatabase) WriteResultChunks(ctx context.Context, resultChunks map[int]types.ResultChunkData) error {
	return batch.WriteResultChunks(ctx, w.store, "result_chunks", w.serializer, resultChunks)
}

func (w *sqliteDatabase) WriteDefinitions(ctx context.Context, monikerLocations []types.MonikerLocations) error {
	return batch.WriteMonikerLocations(ctx, w.store, "definitions", w.serializer, monikerLocations)
}

func (w *sqliteDatabase) WriteReferences(ctx context.Context, monikerLocations []types.MonikerLocations) error {
	return batch.WriteMonikerLocations(ctx, w.store, "references", w.serializer, monikerLocations)
}

func createTables(ctx context.Context, store *store.Store) error {
	queries := []*sqlf.Query{
		sqlf.Sprintf(`CREATE TABLE "schema_version" ("version" text NOT NULL)`),
		sqlf.Sprintf(`CREATE TABLE "meta" ("num_result_chunks" integer NOT NULL)`),
		sqlf.Sprintf(`CREATE TABLE "documents" ("path" text PRIMARY KEY NOT NULL, "data" blob NOT NULL)`),
		sqlf.Sprintf(`CREATE TABLE "result_chunks" ("id" integer PRIMARY KEY NOT NULL, "data" blob NOT NULL)`),
		sqlf.Sprintf(`CREATE TABLE "definitions" ("scheme" text NOT NULL, "identifier" text NOT NULL, "data" blob NOT NULL, PRIMARY KEY (scheme, identifier))`),
		sqlf.Sprintf(`CREATE TABLE "references" ("scheme" text NOT NULL, "identifier" text NOT NULL, "data" blob NOT NULL, PRIMARY KEY (scheme, identifier))`),
	}

	for _, query := range queries {
		if err := store.Exec(ctx, query); err != nil {
			return err
		}
	}

	return nil
}
