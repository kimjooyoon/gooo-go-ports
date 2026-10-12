package main

import (
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"path/filepath"
	"slices"

	fixed "github.com/kimjooyoon/gooo-go-ports/third_party/x-image/math/fixed"
)

const fixedPackage = "go/golang.org/x/image/math/fixed"
const fixedPortBatchSize = 64
const fixedConsumerBatchSize = 32
const fixedSourceSHA = "b74f2166fce03cecfebb00d3e9641b559dab8ffb00d72139c1dc69ed1ac05725"
const fixedLicenseSHA = "911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad"

type fixedObservations struct {
	Suites                                                                               []suite
	PortReferenceCases, PortRejectionCases                                               int
	ConsumerReferenceCases, ConsumerDomainRejectionCases, ConsumerOverflowRejectionCases int
}

// The oracle calls unchanged upstream methods. Its embedded source also binds
// the implementation compiled into this verifier, not merely a later disk copy.
type externalSource struct {
	Module            string            `json:"module"`
	Repository        string            `json:"repository"`
	Revision          string            `json:"revision"`
	Source            string            `json:"source"`
	SourceSHA         string            `json:"source_sha256"`
	OracleCopy        string            `json:"oracle_copy"`
	License           string            `json:"license"`
	LicenseSHA        string            `json:"license_sha256"`
	Target            string            `json:"target"`
	Symbols           []string          `json:"symbols"`
	Version           string            `json:"version,omitempty"`
	ModuleH1          string            `json:"module_h1,omitempty"`
	SupportingSources []string          `json:"supporting_sources,omitempty"`
	SupportingSHA     map[string]string `json:"supporting_source_sha256,omitempty"`
	SupportingTargets []string          `json:"supporting_targets,omitempty"`
}

// The reviewed external source contract is independent of the supplied lock.
// Unknown documentation fields remain permitted, as for the existing lock.
func verifyFixedExternalSources(sources []externalSource) error {
	if len(sources) != 1 {
		return fmt.Errorf("fixed external source roster")
	}
	s := sources[0]
	if s.Module != "golang.org/x/image" || s.Repository != "https://github.com/golang/image" ||
		s.Revision != "b06f1de3f4900ff828b8f114c37eb9ea10dfed90" || s.Source != "math/fixed/fixed.go" ||
		s.SourceSHA != fixedSourceSHA || s.OracleCopy != "third_party/x-image/math/fixed/fixed.go" ||
		s.License != "third_party/x-image/LICENSE" || s.LicenseSHA != fixedLicenseSHA ||
		s.Target != "ports/image/fixed.gooo" || !slices.Equal(s.Symbols, []string{"Int26_6.Floor", "Int26_6.Ceil"}) {
		return fmt.Errorf("fixed external source identity, path, symbols or digest differs")
	}
	return nil
}

func verifyFixedOracle(root string, sources []externalSource) error {
	if err := verifyFixedExternalSources(sources); err != nil {
		return err
	}
	if fixed.PinnedSourceSHA256() != fixedSourceSHA {
		return fmt.Errorf("compiled fixed oracle source differs")
	}
	for name, want := range map[string]string{"third_party/x-image/math/fixed/fixed.go": fixedSourceSHA, "third_party/x-image/LICENSE": fixedLicenseSHA} {
		got, err := digestFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("fixed upstream file differs: %s", name)
		}
	}
	return nil
}

func fixedRejectedInputs() []int64 {
	return []int64{math.MinInt64, -9007199254740993, -4294967296, math.MinInt32 - 1, math.MaxInt32 + 1, 4294967296, 9007199254740993, math.MaxInt64}
}
func fixedRawInputs() []int64 {
	values := []int64{}
	seen := map[int64]bool{}
	add := func(n int64) {
		if !seen[n] {
			seen[n] = true
			values = append(values, n)
		}
	}
	// Every remainder 0..63 for eleven fixed signed quotients, including both
	// int32 extremes. These are finite raw samples, not every int32 input.
	for _, q := range []int64{-33554432, -33554431, -65536, -2, -1, 0, 1, 2, 65536, 33554430, 33554431} {
		for r := int64(0); r < 64; r++ {
			add(q*64 + r)
		}
	}
	rng := rand.New(rand.NewPCG(197, 239))
	for i := 0; i < 128; i++ {
		add(int64(rng.Uint32()) + math.MinInt32)
	}
	return values
}
func fixedPortOracle(n int64, ceil bool) any {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return map[string]any{"in_fixed_domain": false, "value": int64(0)}
	}
	x := fixed.Int26_6(n)
	value := x.Floor()
	if ceil {
		value = x.Ceil()
	}
	return map[string]any{"in_fixed_domain": true, "value": int64(value)}
}
func fixedPortSuite(activity string, inputs [][]any, oracle func(int64, bool) any) (suite, error) {
	if activity != "Int26_6Floor" && activity != "Int26_6Ceil" {
		return suite{}, fmt.Errorf("unknown fixed operation")
	}
	if len(inputs) == 0 {
		return suite{}, fmt.Errorf("empty fixed corpus")
	}
	seen := map[int64]bool{}
	for _, row := range inputs {
		if len(row) != 1 {
			return suite{}, fmt.Errorf("fixed arity")
		}
		n, ok := row[0].(int64)
		if !ok {
			return suite{}, fmt.Errorf("fixed raw input must be original int64")
		}
		if seen[n] {
			return suite{}, fmt.Errorf("duplicate fixed raw input")
		}
		seen[n] = true
	}
	s := suite{Package: fixedPackage, Activity: activity}
	for _, row := range inputs {
		n := row[0].(int64)
		s.Cases = append(s.Cases, testCase{Inputs: []any{n}, Expected: oracle(n, activity == "Int26_6Ceil")})
	}
	return s, nil
}
func fixedPixelResult(accepted bool, r image.Rectangle) map[string]any {
	result := imageRectangleRecord(r)
	result["accepted"] = accepted
	result["has_pixels"] = !r.Empty()
	return result
}

// Domain is checked BEFORE any conversion to upstream Int26_6. Empty original
// selections precede overflow rejection, so fractional lines cannot gain area.
func fixedPixelRejection(raw rectangleCoordinates) string {
	for _, n := range raw {
		if n < math.MinInt32 || n > math.MaxInt32 {
			return "domain"
		}
	}
	if raw[0] >= raw[2] || raw[1] >= raw[3] {
		return ""
	}
	if raw[2] > math.MaxInt32-63 || raw[3] > math.MaxInt32-63 {
		return "ceil-overflow"
	}
	return ""
}
func fixedPixelOracle(rs []image.Rectangle) any {
	raw := rectangleCoordinates{int64(rs[1].Min.X), int64(rs[1].Min.Y), int64(rs[1].Max.X), int64(rs[1].Max.Y)}
	if fixedPixelRejection(raw) != "" {
		return fixedPixelResult(false, image.Rectangle{})
	}
	if rs[1].Empty() {
		return fixedPixelResult(true, image.Rectangle{})
	}
	pixels := image.Rectangle{Min: image.Point{X: fixed.Int26_6(raw[0]).Floor(), Y: fixed.Int26_6(raw[1]).Floor()}, Max: image.Point{X: fixed.Int26_6(raw[2]).Ceil(), Y: fixed.Int26_6(raw[3]).Ceil()}}
	return fixedPixelResult(true, rs[0].Intersect(pixels))
}
func fixedPixelNamedPairs() [][2]rectangleCoordinates {
	b := rectangleCoordinates{-10, -10, 10, 10}
	wide := rectangleCoordinates{math.MinInt64, math.MinInt64, math.MaxInt64, math.MaxInt64}
	return [][2]rectangleCoordinates{
		{b, {32, 32, 96, 96}}, {b, {-65, -1, 1, 65}}, {b, {32, 0, 32, 64}}, {b, {2, 0, 1, 64}},
		{b, {0, 32, 64, 32}}, {b, {0, 2, 64, 1}}, {b, {640, 0, 704, 64}}, {b, {-704, -704, 704, 704}},
		{{10, 10, -10, -10}, {-64, -64, 64, 64}}, {wide, {math.MinInt32, math.MinInt32, math.MaxInt32 - 63, math.MaxInt32 - 63}},
		{wide, {0, 0, math.MaxInt32 - 62, 64}}, {wide, {0, 0, 64, math.MaxInt32 - 62}},
		{wide, {math.MaxInt32, 0, math.MaxInt32, 64}}, {wide, {math.MaxInt32, 0, math.MaxInt32 - 1, 64}},
		{wide, {math.MinInt32 - 1, 0, 0, 0}}, {{9007199254740993, 0, 9007199254740994, 1}, {0, 0, 64, 64}},
	}
}
func fixedPixelInputs() [][]any {
	inputs := [][]any{}
	seen := map[[8]int64]bool{}
	add := func(b, r rectangleCoordinates) {
		var key [8]int64
		copy(key[:4], b[:])
		copy(key[4:], r[:])
		if !seen[key] {
			seen[key] = true
			inputs = append(inputs, []any{rectangleRecord(b), rectangleRecord(r)})
		}
	}
	for _, p := range fixedPixelNamedPairs() {
		add(p[0], p[1])
	}
	b := rectangleCoordinates{-10, -10, 10, 10}
	for _, r := range geometryGrid([]int64{-1, 0, 1}) {
		add(b, r)
	}
	for r := int64(0); r < 64; r++ {
		for pos := 0; pos < 4; pos++ {
			raw := rectangleCoordinates{-128, -128, 128, 128}
			raw[pos] += r
			add(b, raw)
		}
	}
	for _, n := range append([]int64{math.MinInt32, math.MinInt32 + 1, math.MaxInt32 - 64, math.MaxInt32 - 63, math.MaxInt32 - 62, math.MaxInt32}, fixedRejectedInputs()...) {
		for pos := 0; pos < 4; pos++ {
			raw := rectangleCoordinates{0, 0, 64, 64}
			raw[pos] = n
			add(b, raw)
		}
	}
	for _, n := range geometryBoundaryValues() {
		for pos := 0; pos < 4; pos++ {
			bounds := b
			bounds[pos] = n
			add(bounds, rectangleCoordinates{-65, -65, 65, 65})
		}
	}
	rng := rand.New(rand.NewPCG(251, 283))
	for i := 0; i < 64; i++ {
		var raw rectangleCoordinates
		for j := range raw {
			raw[j] = int64(rng.Uint32()) + math.MinInt32
		}
		add(b, raw)
	}
	return inputs
}
func fixedCorpus() (fixedObservations, error) {
	result := fixedObservations{}
	raw := fixedRawInputs()
	inputs := [][]any{}
	for _, n := range append(append([]int64{}, raw...), fixedRejectedInputs()...) {
		inputs = append(inputs, []any{n})
	}
	for _, name := range []string{"Int26_6Floor", "Int26_6Ceil"} {
		s, err := fixedPortSuite(name, inputs, fixedPortOracle)
		if err != nil {
			return result, err
		}
		s.Domain = fmt.Sprintf("original signed int32 raw Q26.6; %d finite unchanged x/image Go method observations, including all 64 residues at eleven quotients, int32 overflow neighbors and fixed PCG samples; %d separate outside-int32 rejection observations. Result record is an explicit API change; source int32 overflow is preserved. Not a 2^32 exhaustive check", len(raw), len(fixedRejectedInputs()))
		result.Suites = append(result.Suites, s)
		result.PortReferenceCases += len(raw)
		result.PortRejectionCases += len(fixedRejectedInputs())
	}
	s, err := geometrySuite("example/fixed-pixel-bounds", "PixelBounds", 2, fixedPixelInputs(), fixedPixelOracle)
	if err != nil {
		return result, err
	}
	for _, row := range s.Cases {
		raw, _ := readRectangle(row.Inputs[1])
		switch fixedPixelRejection(raw) {
		case "domain":
			result.ConsumerDomainRejectionCases++
		case "ceil-overflow":
			result.ConsumerOverflowRejectionCases++
		default:
			result.ConsumerReferenceCases++
		}
	}
	s.Domain = fmt.Sprintf("original int64 image bounds plus raw Q26.6 rectangle; %d finite Go-composition observations, %d original-raw-domain rejections, %d nonempty Ceil-overflow rejections. Raw empty/reversed selection is checked before rounding; bounds remain unnormalized. Coordinates only, no rendering, pixel processing or exhaustive domain claim", result.ConsumerReferenceCases, result.ConsumerDomainRejectionCases, result.ConsumerOverflowRejectionCases)
	result.Suites = append(result.Suites, s)
	return result, nil
}
