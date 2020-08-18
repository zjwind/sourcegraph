package sqlite

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/bundles/persistence/cache"
)

func MergeDBs(ctx context.Context, baseBundleFile, patchBundleFile, resultBundleFile string) error {
	tempDir, err := ioutil.TempDir("","")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	tempBundleFile := filepath.Join(tempDir, "tempBundle")

	err = func () error {
		tempBundleWriter, err := os.OpenFile(tempBundleFile, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		defer tempBundleWriter.Close()
		baseBundleReader, err := os.Open(baseBundleFile)
		if err != nil {
			return err
		}
		defer baseBundleReader.Close()

		_, err = io.Copy(tempBundleWriter, baseBundleReader)
		return err
	}()
	if err != nil {
		return err
	}

	cache, err := cache.NewDataCache(1)
	tempBundleDB, err := NewDatabase(ctx, tempBundleFile, cache)
	patchBundleDB, err := NewReader(ctx, patchBundleFile, cache)
	err = tempBundleDB.PatchDatabase(ctx, patchBundleDB)

	return os.Rename(tempBundleFile, resultBundleFile)
}
