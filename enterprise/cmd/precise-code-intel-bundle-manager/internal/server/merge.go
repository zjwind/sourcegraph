package server


func (db *databaseImpl) DocumentsReferencing(ctx context.Context, paths []string) ([]string, error) {
	pathMap := map[string]struct{}{}
	for _, path := range paths {
		pathMap[path] = struct{}{}
	}

	resultIDs := map[types.ID]struct{}{}
	for i := 0; i < db.numResultChunks; i++ {
		resultChunk, exists, err := db.getResultChunkByID(ctx, i)
		if err != nil {
			return nil, err
		}
		if !exists {
			// TODO(efritz) - document that this should be fine
			continue
		}

		for resultID, documentIDRangeIDs := range resultChunk.DocumentIDRangeIDs {
			for _, documentIDRangeID := range documentIDRangeIDs {
				// Skip results that do not point into one of the given documents
				if _, ok := pathMap[resultChunk.DocumentPaths[documentIDRangeID.DocumentID]]; !ok {
					continue
				}

				resultIDs[resultID] = struct{}{}
			}
		}
	}

	allPaths, err := db.reader.PathsWithPrefix(ctx, "")
	if err != nil {
		return nil, err
	}

	var pathsReferencing []string
	for _, path := range allPaths {
		document, exists, err := db.getDocumentData(ctx, path)
		if err != nil {
			return nil, err
		}
		if !exists {
			// TODO(efritz) - add error here - document should definitely exist
			continue
		}

		for _, r := range document.Ranges {
			if _, ok := resultIDs[r.DefinitionResultID]; ok {
				pathsReferencing = append(pathsReferencing, path)
				break
			}
		}
	}

	return pathsReferencing, nil
}
