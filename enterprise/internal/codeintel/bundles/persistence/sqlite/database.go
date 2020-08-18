package sqlite

import (
	"context"
	"os"
	"sort"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/cache"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/serialization"
	gobserializer "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/serialization/gob"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/sqlite/migrate"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/sqlite/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/types"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/util"
)

type sqliteDatabase struct {
	filename   string
	cache      cache.DataCache
	store      *store.Store
	closer     func() error
	serializer serialization.Serializer
}

var _ persistence.Database = &sqliteDatabase{}

func NewDatabase(ctx context.Context, filename string, cache cache.DataCache) (persistence.Database, error) {
	newDB := false
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		newDB = true
	}
	store, closer, err := store.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if closeErr := closer(); closeErr != nil {
				err = multierror.Append(err, closeErr)
			}
		}
	}()

	store, err = store.Transact(ctx)
	if err != nil {
		return nil, err
	}

	serializer := gobserializer.New()

	if newDB {
		if err := createTables(ctx, store); err != nil {
			return nil, err
		}
	} else {
		if err := migrate.Migrate(ctx, store, serializer); err != nil {
			return nil, err
		}
	}

	return &sqliteDatabase{
		filename:   filename,
		cache:      cache,
		store:      store,
		closer:     closer,
		serializer: serializer,
	}, nil
}

func (base *sqliteDatabase) PatchDatabase(ctx context.Context, patch persistence.Reader) (err error) {
	patchPaths, err := patch.PathsWithPrefix(ctx, "")
	patchMeta, err := patch.ReadMeta(ctx)

	patchDocuments := make(map[string]types.DocumentData)
	for _, path := range patchPaths {
		document, _, _ := patch.ReadDocument(ctx, path)
		patchDocuments[path] = document
	}

	patchResultChunks := make(map[int]types.ResultChunkData)
	for id := 0; id < patchMeta.NumResultChunks; id++ {
		resultChunk, _, _ := patch.ReadResultChunk(ctx, id)
		patchResultChunks[id] = resultChunk
	}

	basePaths, err := base.PathsWithPrefix(ctx, "")
	baseMeta, err := base.ReadMeta(ctx)

	baseDocuments := make(map[string]types.DocumentData)
	for _, path := range basePaths {
		document, _, _ := base.ReadDocument(ctx, path)
		baseDocuments[path] = document
	}

	baseResultChunks := make(map[int]types.ResultChunkData)
	for id := 0; id < baseMeta.NumResultChunks; id++ {
		resultChunk, _, _ := base.ReadResultChunk(ctx, id)
		baseResultChunks[id] = resultChunk
	}

	var reindexedPaths, deletedPaths, modifiedPaths, indexedPaths map[string]struct{}
	deletedRefs := make(map[types.ID]struct{})

	for documentPath := range modifiedPaths {
		for _, rng := range baseDocuments[documentPath].Ranges {
			if _, exists := deletedRefs[rng.ReferenceResultID]; exists {
				continue
			}
			refChunkId := types.HashKey(rng.ReferenceResultID, baseMeta.NumResultChunks)
			refChunk := baseResultChunks[refChunkId]
			refs := refChunk.DocumentIDRangeIDs[rng.ReferenceResultID]
			var filteredRefs []types.DocumentIDRangeID
			for _, ref := range refs {
				documentPath := refChunk.DocumentPaths[ref.DocumentID]
				if _, exists := modifiedPaths[documentPath]; !exists {
					filteredRefs = append(filteredRefs, ref)
				}
			}
			refChunk.DocumentIDRangeIDs[rng.ReferenceResultID] = filteredRefs
			deletedRefs[rng.ReferenceResultID] = struct{}{}
		}
	}

	for documentPath := range reindexedPaths {
		delete(baseDocuments, documentPath)
	}
	for documentPath := range deletedPaths {
		delete(baseDocuments, documentPath)
	}

	defResultsByPath := make(map[string]map[types.ID]types.RangeData)

	for path := range indexedPaths {
		patchDocument := patchDocuments[path]
		for _, rng := range patchDocument.Ranges {
			if rng.DefinitionResultID != "" {
				defChunkId := types.HashKey(rng.DefinitionResultID, patchMeta.NumResultChunks)
				defChunk := baseResultChunks[defChunkId]
				defLoc := defChunk.DocumentIDRangeIDs[rng.DefinitionResultID][0]
				defPath := defChunk.DocumentPaths[defLoc.DocumentID]
				def := patchDocuments[defPath].Ranges[defLoc.RangeID]
				defResults, ok := defResultsByPath[defPath]
				if !ok {
					defResults = make(map[types.ID]types.RangeData)
					defResultsByPath[defPath] = defResults
				}
				defResults[defLoc.RangeID] = def
			}
		}
	}

	for path, defsMap := range defResultsByPath {
		var baseRngs []types.RangeData
		for _, rng := range baseDocuments[path].Ranges {
			baseRngs = append(baseRngs, rng)
		}
		var defsSlice []types.RangeData
		for _, rng := range defsMap {
			defsSlice = append(defsSlice, rng)
		}
		for _, rngSlice := range [][]types.RangeData{baseRngs, defsSlice} {
			sort.Slice(rngSlice, func(i, j int) bool {
				return util.CompareRanges(rngSlice[i], rngSlice[j]) < 0
			})
		}
		var baseIdx int
		baseRng := baseRngs[baseIdx]
		for _, def := range defsSlice {
			for ; util.CompareRanges(baseRng, def) < 0; baseIdx++ {
				baseRng = baseRngs[baseIdx + 1]
			}

			patchChunkId := types.HashKey(def.ReferenceResultID, patchMeta.NumResultChunks)
			patchChunk := patchResultChunks[patchChunkId]
			patchRefs := patchChunk.DocumentIDRangeIDs[def.ReferenceResultID]

			baseReferenceResultID := baseRng.ReferenceResultID
			if util.CompareRanges(baseRng, def) != 0 {
				id, err := uuid.NewRandom()
				if err != nil {
					return err
				}
				baseReferenceResultID = types.ID(id.String())
			}
			baseChunkId := types.HashKey(baseReferenceResultID, baseMeta.NumResultChunks)
			baseChunk := baseResultChunks[baseChunkId]
			baseRefs := baseChunk.DocumentIDRangeIDs[baseReferenceResultID]

			baseDocumentIDs := make(map[string]types.ID)
			for id, path := range baseChunk.DocumentPaths {
				baseDocumentIDs[path] = id
			}
			for _, ref := range patchRefs {
				path := patchChunk.DocumentPaths[ref.DocumentID]
				baseDocumentID, exists := baseDocumentIDs[path]
				if !exists {
					id, err := uuid.NewRandom()
					if err != nil {
						return err
					}
					baseDocumentID = types.ID(id.String())
					baseDocumentIDs[path] = baseDocumentID
					baseChunk.DocumentPaths[baseDocumentID] = path
				}
				ref.DocumentID = baseDocumentID
				baseRefs = append(baseRefs, ref)
				rng := baseDocuments[path].Ranges[ref.RangeID]
				rng.ReferenceResultID = baseReferenceResultID
			}
			baseChunk.DocumentIDRangeIDs[baseReferenceResultID] = baseRefs
		}
	}

	for path := range indexedPaths {
		baseDocuments[path] = patchDocuments[path]
	}

	err = base.WriteDocuments(ctx, baseDocuments)
	err = base.WriteResultChunks(ctx, baseResultChunks)

	return err
}

func (db *sqliteDatabase) Close(err error) error {
	err = db.store.Done(err)

	if closeErr := db.closer(); closeErr != nil {
		err = multierror.Append(err, closeErr)
		return err
	}

	return nil
}
