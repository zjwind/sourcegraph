package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/google/go-cmp/cmp"
	"github.com/inconshreveable/log15"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/conversion"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/lsif/validation"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

type projectResult struct {
	name       string
	usage      usageStats
	output     string
	testResult testSuiteResult
}

type usageStats struct {
	// Memory usage in kilobytes by child process.
	memory int64
}

type passedTest struct {
	Name string
}

type failedTest struct {
	Name string
	Diff string
}

type testFileResult struct {
	Name   string
	Passed []passedTest
	Failed []failedTest
}

type testSuiteResult struct {
	FileResults []testFileResult
}

var directory string
var raw_indexer string
var debug bool

// TODO: Do more monitoring of the process.
// var monitor bool

func logFatal(msg string, args ...interface{}) {
	log15.Error(msg, args...)
	os.Exit(1)
}

func main() {
	flag.StringVar(&directory, "dir", ".", "The directory to run the test harness over")
	flag.StringVar(&raw_indexer, "indexer", "", "The name of the indexer that you want to test")
	flag.BoolVar(&debug, "debug", false, "Enable debugging")

	flag.Parse()

	if debug {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlDebug, log15.StdoutHandler))
	} else {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlError, log15.StdoutHandler))
	}

	if raw_indexer == "" {
		logFatal("Indexer is required. Pass with --indexer")
	}

	log15.Info("Starting Execution: ", "directory", directory, "indexer", raw_indexer)

	indexer := strings.Split(raw_indexer, " ")
	if err := testDirectory(context.Background(), indexer, directory); err != nil {
		logFatal("Failed with", "err", err)
	}
	log15.Info("Tests passed:", "directory", directory, "indexer", raw_indexer)
}

func testDirectory(ctx context.Context, indexer []string, directory string) error {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return err
	}

	type channelResult struct {
		result projectResult
		err    error
	}

	resultChan := make(chan channelResult, len(files))
	var wg sync.WaitGroup

	for _, f := range files {
		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			projResult, err := testProject(ctx, indexer, path.Join(directory, name), name)

			resultChan <- channelResult{
				result: projResult,
				err:    err,
			}
		}(f.Name())

	}

	wg.Wait()
	close(resultChan)

	allPassed := true
	for chanResult := range resultChan {
		if chanResult.err != nil {
			log15.Warn("Failed to run test. Got err:", "error", chanResult.err)
			continue
		}

		for _, fileResult := range chanResult.result.testResult.FileResults {
			if len(fileResult.Failed) > 0 {
				allPassed = false
				for _, failed := range fileResult.Failed {
					fmt.Printf("Failed test: File: %s, Name: %s\nDiff: %s\n", fileResult.Name, failed.Name, failed.Diff)
				}
			}
		}
	}

	if !allPassed {
		return errors.New("Failed some tests. Try again later :)")
	}

	return nil
}

func testProject(ctx context.Context, indexer []string, project, name string) (projectResult, error) {
	output, err := setupProject(project)
	if err != nil {
		return projectResult{name: name, output: string(output)}, err
	}

	log15.Debug("... Completed setup project", "command", indexer)
	result, err := runIndexer(ctx, indexer, project, name)
	if err != nil {
		return projectResult{
			name:   name,
			output: result.output,
		}, err
	}

	log15.Debug("... \t Resource Usage:", "usage", result.usage)

	if err := validateDump(project); err != nil {
		fmt.Println("ERROR:", err)
		// return projectResult{}, err
	}
	log15.Debug("... Validated dump.lsif")

	bundle, err := readBundle(project)
	if err != nil {
		return projectResult{name: name}, err
	}

	testResult, err := validateTestCases(project, bundle)
	if err != nil {
		return projectResult{name: name}, err
	}

	return projectResult{
		name:       name,
		usage:      result.usage,
		output:     string(output),
		testResult: testResult,
	}, nil
}

func setupProject(directory string) ([]byte, error) {
	cmd := exec.Command("./setup_indexer.sh")
	cmd.Dir = directory

	return cmd.CombinedOutput()
}

func runIndexer(ctx context.Context, indexer []string, directory, name string) (projectResult, error) {
	command := indexer[0]
	args := indexer[1:]

	log15.Debug("... Generating dump.lsif")
	cmd := exec.Command(command, args...)
	cmd.Dir = directory

	output, err := cmd.CombinedOutput()
	if err != nil {
		return projectResult{}, err
	}

	sysUsage := cmd.ProcessState.SysUsage()
	mem, _ := MaxMemoryInKB(sysUsage)

	return projectResult{
		name:   name,
		usage:  usageStats{memory: mem},
		output: string(output),
	}, err
}

func validateDump(directory string) error {
	dumpFile, err := os.Open(filepath.Join(directory, "dump.lsif"))
	if err != nil {
		return err
	}

	ctx := validation.NewValidationContext()
	validator := &validation.Validator{Context: ctx}

	if err := validator.Validate(dumpFile); err != nil {
		return err
	}

	if len(ctx.Errors) > 0 {
		msg := fmt.Sprintf("Detected %d errors", len(ctx.Errors))
		for i, err := range ctx.Errors {
			msg += fmt.Sprintf("%d. %s", i, err)
		}
		return errors.New(msg)
	}

	return nil
}

func validateTestCases(projectRoot string, bundle *semantic.GroupedBundleDataMaps) (testSuiteResult, error) {
	testFiles, err := os.ReadDir(filepath.Join(projectRoot, "lsif_tests"))
	if err != nil {
		if os.IsNotExist(err) {
			log15.Warn("No lsif test directory exists here", "directory", projectRoot)
			return testSuiteResult{}, nil
		}

		return testSuiteResult{}, err
	}

	fileResults := []testFileResult{}
	for _, file := range testFiles {
		if testFileExtension := filepath.Ext(file.Name()); testFileExtension != ".json" {
			continue
		}

		testFileName := filepath.Join(projectRoot, "lsif_tests", file.Name())
		fileResult, err := runOneTestFile(projectRoot, testFileName, bundle)
		if err != nil {
			logFatal("Had an error while we do the test file", "file", testFileName, "err", err)
		}

		fileResults = append(fileResults, fileResult)
	}

	return testSuiteResult{FileResults: fileResults}, nil
}

func runOneTestFile(projectRoot, file string, bundle *semantic.GroupedBundleDataMaps) (testFileResult, error) {
	doc, err := ioutil.ReadFile(file)
	if err != nil {
		return testFileResult{}, errors.Wrap(err, "Failed to read file")
	}

	var testCase LsifTest
	if err := json.Unmarshal(doc, &testCase); err != nil {
		return testFileResult{}, errors.Wrap(err, "Malformed JSON")
	}

	fileResult := testFileResult{Name: file}

	for _, definitionTest := range testCase.Definitions {
		if err := runOneDefinitionRequest(projectRoot, bundle, definitionTest, &fileResult); err != nil {
			return fileResult, err
		}
	}

	for _, referencesTest := range testCase.References {
		if err := runOneReferencesRequest(projectRoot, bundle, referencesTest, &fileResult); err != nil {
			return fileResult, err
		}
	}

	return fileResult, nil
}

func sortPosition(left, right Position) int {
	if left.Line > right.Line {
		return -1
	} else if left.Line < right.Line {
		return 1
	}

	if left.Character > right.Character {
		return -1
	} else if left.Character < right.Character {
		return 1
	}

	return 0
}

func sortRange(left, right Range) int {
	start := sortPosition(left.Start, right.Start)
	if start != 0 {
		return start
	}

	end := sortPosition(left.End, right.End)
	if end != 0 {
		return end
	}

	return 0
}

func sortReferences(references []Location) func(i, j int) bool {
	return func(i, j int) bool {
		left := references[i]
		right := references[j]

		if left.URI > right.URI {
			return false
		} else if left.URI < right.URI {
			return true
		}

		cmpRange := sortRange(left.Range, right.Range)
		if cmpRange != 0 {
			return cmpRange > 0
		}

		return i < j
	}
}

func runOneReferencesRequest(projectRoot string, bundle *semantic.GroupedBundleDataMaps, testCase ReferencesTest, fileResult *testFileResult) error {
	request := testCase.Request

	filePath := request.TextDocument
	line := request.Position.Line
	character := request.Position.Character

	results, err := semantic.Query(bundle, filePath, line, character)
	if err != nil {
		return err
	}

	// TODO: We need to add support for not including the declaration from the context.
	//       I don't know of any way to do that currently, so it would require changes to Query or similar.
	if !request.Context.IncludeDeclaration {
		return errors.New("'context.IncludeDeclaration = false' configuration is not currently supported")
	}

	// TODO: I'm not sure when we'd send more than one result, rather than multiple references for this.
	if len(results) > 1 {
		return errors.New("Had too many results")
	} else if len(results) == 0 {
		return errors.New("Found no results")
	}

	semanticReferences := results[0].References

	actualReferences := make([]Location, len(semanticReferences))
	for index, ref := range semanticReferences {
		actualReferences[index] = transformLocationToResponse(ref)
	}

	expectedReferences := []Location(testCase.Response)

	sort.SliceStable(actualReferences, sortReferences(actualReferences))
	sort.SliceStable(expectedReferences, sortReferences(expectedReferences))

	filesToContents := make(map[string]string)

	if !cmp.Equal(actualReferences, expectedReferences) {
		diff := ""
		for index, actual := range actualReferences {
			expected := expectedReferences[index]
			if actual == expected {
				continue
			}

			contents, ok := filesToContents[actual.URI]
			if !ok {
				// ok, read the file
				fileName := strings.Replace(actual.URI, "file://", "", 1)
				byteContents, err := os.ReadFile(path.Join(projectRoot, fileName))
				if err != nil {
					return err
				}

				contents = string(byteContents)
				filesToContents[actual.URI] = contents
			}

			thisDiff, err := DrawLocations(contents, expected, actual)
			if err != nil {
				return errors.Wrap(err, "Unable to draw the pretty diff")
			}

			diff += cmp.Diff(actual, expected)
			diff += "\n" + thisDiff
		}

		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: diff,
		})
	} else {
		fileResult.Passed = append(fileResult.Passed, passedTest{
			Name: testCase.Name,
		})
	}

	return nil
}

func runOneDefinitionRequest(projectRoot string, bundle *semantic.GroupedBundleDataMaps, testCase DefinitionTest, fileResult *testFileResult) error {
	request := testCase.Request

	path := request.TextDocument
	line := request.Position.Line
	character := request.Position.Character

	results, err := semantic.Query(bundle, path, line, character)
	if err != nil {
		return err
	}

	// TODO: We probably can have more than one result and have that make sense...
	//		should allow testing that
	if len(results) > 1 {
		return errors.New("Had too many results")
	} else if len(results) == 0 {
		return errors.New("Found no results")
	}

	definitions := results[0].Definitions

	if len(definitions) > 1 {
		logFatal("Had too many definitions", "definitions", definitions)
	} else if len(definitions) == 0 {
		logFatal("Found no definitions", "definitions", definitions)
	}

	response := transformLocationToResponse(definitions[0])
	if diff := cmp.Diff(response, testCase.Response); diff != "" {
		fileResult.Failed = append(fileResult.Failed, failedTest{
			Name: testCase.Name,
			Diff: diff,
		})
	} else {
		fileResult.Passed = append(fileResult.Passed, passedTest{
			Name: testCase.Name,
		})
	}

	return nil
}

func transformLocationToResponse(location semantic.LocationData) Location {
	return Location{
		URI: "file://" + location.URI,
		Range: Range{
			Start: Position{
				Line:      location.StartLine,
				Character: location.StartCharacter,
			},
			End: Position{
				Line:      location.EndLine,
				Character: location.EndCharacter,
			},
		},
	}

}
func readBundle(root string) (*semantic.GroupedBundleDataMaps, error) {
	bundle, err := conversion.CorrelateLocalGitRelative(context.Background(), path.Join(root, "dump.lsif"), root)
	if err != nil {
		return nil, err
	}

	return semantic.GroupedBundleDataChansToMaps(bundle), nil
}
