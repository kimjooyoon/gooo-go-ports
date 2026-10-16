package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
)

const uuidRevision = "0f11ee6918f41a04c201eceeadf612a377bc7fbc"
const uuidModuleH1 = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0="
const uuidSourceSHA = "0edec8e34c6b6fe0db31b71a29069a09ed832e3fd04ee0175916b58f2b60e5c1"
const uuidUtilSHA = "755f50afafb13dd89ae6177da7eb6dc31ae650ece2f9ba52821021e9f8de044e"
const uuidLicenseSHA = "0a8d61ed3cbfd5312326e8126c31ce9c627a283adc99131b56896d29ada04b2d"

// Exactly the upstream module's 31 archive files, excluding our separately
// identified hash wrapper and provenance notes. This list is independent of
// editable metadata. Extra owned files never change the claimed upstream h1.
var uuidOriginalFiles = []string{
	".github/CODEOWNERS",
	".github/release-please.yml",
	".github/workflows/apidiff.yaml",
	".github/workflows/tests.yaml",
	"CHANGELOG.md",
	"CONTRIBUTING.md",
	"CONTRIBUTORS",
	"LICENSE",
	"README.md",
	"dce.go",
	"doc.go",
	"go.mod",
	"hash.go",
	"json_test.go",
	"marshal.go",
	"node.go",
	"node_js.go",
	"node_net.go",
	"null.go",
	"null_test.go",
	"seq_test.go",
	"sql.go",
	"sql_test.go",
	"time.go",
	"util.go",
	"uuid.go",
	"uuid_test.go",
	"version1.go",
	"version4.go",
	"version6.go",
	"version7.go",
}

func externalSourcesForModule(sources []externalSource, module string) []externalSource {
	var result []externalSource
	for _, s := range sources {
		if s.Module == module {
			result = append(result, s)
		}
	}
	return result
}

func verifyExternalOracles(root string, sources []externalSource) error {
	if len(sources) != 4 {
		return fmt.Errorf("external oracle module roster")
	}
	for _, s := range sources {
		if s.Module != "golang.org/x/image" && s.Module != "github.com/google/uuid" && s.Module != "golang.org/x/net" {
			return fmt.Errorf("unknown external oracle module %q", s.Module)
		}
	}
	if err := verifyFixedOracle(root, externalSourcesForModule(sources, "golang.org/x/image")); err != nil {
		return err
	}
	if err := verifyUUIDOracle(root, externalSourcesForModule(sources, "github.com/google/uuid")); err != nil {
		return err
	}
	return verifyHTTPGutsOracle(root, externalSourcesForModule(sources, "golang.org/x/net"))
}

const httpgutsNetVersion = "v0.59.0"
const httpgutsNetRevision = "540d04cfe5028e2655754591a4d3e08c586809f2"
const httpgutsNetModuleH1 = "h1:5zfYln+w5XCxwrnMMJPufRgNoXEaGxl0wo5GqPXyues="
const httpgutsTextVersion = "v0.42.0"
const httpgutsTextModuleH1 = "h1:JbOZXgfeCPU9gacVtYliJqOhD+zhrEqK4LfdpmlUZqI="
const httpgutsSourceSHA = "8cfd5819dc821d45f82fde10afff4b2e652535b0091e0559fc8ad7766051a2c2"
const httpgutsGutsSHA = "3da54270a625540809f2f767e54e18acf27199aca64b6cf16d6d3fa3999de959"
const httpgutsTestSHA = "0b16792c7042fe4b955d9335cf8ffe895f4b32d857c15244a8e0801c7081dba0"
const httpgutsLicenseSHA = "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"

// These files are byte-preserved source references, not a copied predicate
// implementation. The oracle imports the actual authenticated module package.
func verifyHTTPGutsOracle(root string, sources []externalSource) error {
	if len(sources) != 1 {
		return fmt.Errorf("httpguts external source roster")
	}
	s := sources[0]
	if s.Module != "golang.org/x/net" || s.Repository != "https://github.com/golang/net" || s.Version != httpgutsNetVersion || s.Revision != httpgutsNetRevision || s.ModuleH1 != httpgutsNetModuleH1 || s.Source != "http/httpguts/httplex.go" || s.SourceSHA != httpgutsSourceSHA || s.OracleCopy != "third_party/x-net/http/httpguts/httplex.go.txt" || s.License != "third_party/x-net/LICENSE" || s.LicenseSHA != httpgutsLicenseSHA || s.Target != "ports/net/httpguts/headers.gooo" || !slices.Equal(s.Symbols, []string{"ValidHeaderFieldName", "ValidHeaderFieldValue"}) || !slices.Equal(s.SupportingSources, []string{"http/httpguts/guts.go", "http/httpguts/httplex_test.go"}) || !reflect.DeepEqual(s.SupportingSHA, map[string]string{"http/httpguts/guts.go": httpgutsGutsSHA, "http/httpguts/httplex_test.go": httpgutsTestSHA}) || len(s.SupportingTargets) != 0 {
		return fmt.Errorf("httpguts module/source/license/symbol metadata differs")
	}
	for name, want := range map[string]string{
		"http/httpguts/httplex.go.txt":      httpgutsSourceSHA,
		"http/httpguts/guts.go.txt":         httpgutsGutsSHA,
		"http/httpguts/httplex_test.go.txt": httpgutsTestSHA,
		"LICENSE":                           httpgutsLicenseSHA,
	} {
		got, err := digestFile(filepath.Join(root, "third_party/x-net", name))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("httpguts original source/license differs: %s", name)
		}
	}
	return nil
}

// This pure gate can be tested without depending on go test's own build info.
// Runtime callers must additionally authenticate/verify whole downloaded
// modules before building; a reported h1 alone is not source authentication.
func verifyHTTPGutsBuildDependencies(info *debug.BuildInfo) error {
	if info == nil {
		return fmt.Errorf("httpguts oracle build dependencies unobserved")
	}
	want := map[string][2]string{
		"golang.org/x/net":  {httpgutsNetVersion, httpgutsNetModuleH1},
		"golang.org/x/text": {httpgutsTextVersion, httpgutsTextModuleH1},
	}
	seen := map[string]bool{}
	for _, dep := range info.Deps {
		if dep == nil {
			continue
		}
		expected, relevant := want[dep.Path]
		if !relevant {
			continue
		}
		if seen[dep.Path] || dep.Version != expected[0] || dep.Sum != expected[1] || dep.Replace != nil {
			return fmt.Errorf("httpguts oracle linked dependency differs: %s", dep.Path)
		}
		seen[dep.Path] = true
	}
	if len(seen) != len(want) {
		return fmt.Errorf("httpguts oracle linked net/text dependencies missing")
	}
	return nil
}

func verifyHTTPGutsRuntimeDependencies() error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("httpguts oracle runtime build info missing")
	}
	return verifyHTTPGutsBuildDependencies(info)
}

func verifyUUIDOracle(root string, sources []externalSource) error {
	if len(sources) != 2 {
		return fmt.Errorf("UUID external source roster")
	}
	// Exactly two reviewed adaptations share one original module. Both rows
	// independently bind immutable source identity; duplicate targets cannot
	// substitute for the Validate or Text ParseCanonical contract.
	targets := map[string]struct{ symbols, supportingTargets []string }{
		"ports/google/uuid/validate.gooo":             {[]string{"Validate"}, []string{"ports/google/uuid/quote.gooo"}},
		"ports/google/uuid/text/parse_canonical.gooo": {[]string{"Parse", "UUID.String"}, []string{"ports/google/uuid/validate.gooo", "ports/google/uuid/quote.gooo"}},
	}
	seen := map[string]bool{}
	for _, s := range sources {
		target, known := targets[s.Target]
		if !known || seen[s.Target] {
			return fmt.Errorf("UUID target roster differs")
		}
		seen[s.Target] = true
		if s.Module != "github.com/google/uuid" || s.Repository != "https://github.com/google/uuid" || s.Version != "v1.6.0" || s.Revision != uuidRevision || s.ModuleH1 != uuidModuleH1 || s.Source != "uuid.go" || s.SourceSHA != uuidSourceSHA || s.OracleCopy != "third_party/google-uuid/uuid.go" || s.License != "third_party/google-uuid/LICENSE" || s.LicenseSHA != uuidLicenseSHA || !slices.Equal(s.Symbols, target.symbols) || !slices.Equal(s.SupportingSources, []string{"util.go"}) || !reflect.DeepEqual(s.SupportingSHA, map[string]string{"util.go": uuidUtilSHA}) || !slices.Equal(s.SupportingTargets, target.supportingTargets) {
			return fmt.Errorf("UUID source identity, paths, symbols or digests differ")
		}
	}

	expected := map[string]string{"uuid.go": uuidSourceSHA, "util.go": uuidUtilSHA, "LICENSE": uuidLicenseSHA}
	if !reflect.DeepEqual(uuid.PinnedSourceSHA256(), expected) {
		return fmt.Errorf("compiled original UUID oracle source differs")
	}
	for name, want := range expected {
		got, err := digestFile(filepath.Join(root, "third_party/google-uuid", name))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("UUID original source or license differs: %s", name)
		}
	}
	// Go dirhash.Hash1's canonical module-archive file names and SHA256 rows.
	var lines strings.Builder
	for _, name := range uuidOriginalFiles {
		raw, err := os.ReadFile(filepath.Join(root, "third_party/google-uuid", name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&lines, "%x  github.com/google/uuid@v1.6.0/%s\n", sha256.Sum256(raw), name)
	}
	sum := sha256.Sum256([]byte(lines.String()))
	if "h1:"+base64.StdEncoding.EncodeToString(sum[:]) != uuidModuleH1 {
		return fmt.Errorf("original UUID module h1 differs")
	}
	return nil
}
