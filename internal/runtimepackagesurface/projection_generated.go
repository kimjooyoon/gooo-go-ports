package runtimepackagesurface

//gooo:generated:start id="urn:gooo:runtime-package-surface:assessment" kind="entity"
type GoooRecordb84d903fa73b073b6352e4a20b21ea24d7ef186b44c53136421a216a22dbc488 struct {
	GoooFielde0569a41080f6be27c724234db35b2e0ddaa768ecffdd83f6a9600dae513054a string   `json:"status"`
	GoooFieldce3661ab6a0d747b227aa8f2d94e352d7a8c877b8560bbbf0978bebca863961d string   `json:"observation_origin"`
	GoooField66ecd7de70125daddd236872bea8837c5b2ad06921b229e3f870ad06364b15f2 []string `json:"declared_paths"`
	GoooField403e560f053ca4b4c83f5b8fa5b2dc06903cdf5a948dbf673c55db79c3c0a966 []string `json:"observed_paths"`
	GoooField6426480fdd193f28b440299b1fd9a0a0305719d7ccf70c5cbb0b0c5bf50edbb2 int64    `json:"missing_count"`
	GoooField7f4697caa2416ae41bbaebbc0880025422a474f132aa4676e3c164502cc3f09b int64    `json:"undeclared_count"`
	GoooField35f06f6ed7bda307c2758805af8ea14aaf9d719ccc55e4c953edeaeb68be275d bool     `json:"duplicate_observation"`
}

//gooo:generated:end id="urn:gooo:runtime-package-surface:assessment" kind="entity"
//gooo:generated:start id="urn:gooo:runtime-package-surface:observation" kind="entity"
type GoooRecord0d8c19ab5f34a1a9ba068ee5a9c2824812be0131a1901904c711675bb7a57c6a struct {
	GoooField00b8d5090334be4e64baac634f42cafc0982183f09eb4423590d1df35161f236 string   `json:"origin_ref"`
	GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314 []string `json:"import_paths"`
}

//gooo:generated:end id="urn:gooo:runtime-package-surface:observation" kind="entity"
//gooo:generated:start id="urn:gooo:runtime-package-surface:paths" kind="entity"
type GoooRecord17467d60761371b1e076a80d4e9915f496bbc5c8f4a0220816a3e04418edf315 struct {
	GoooField624ca5e4c9ce3df2bea4a3edd17a5a16c65059db25a432cb0480135296ac478b []string `json:"values"`
}

//gooo:generated:end id="urn:gooo:runtime-package-surface:paths" kind="entity"

//gooo:generated:start id="urn:gooo:runtime-package-surface:inspect" kind="activity"
func GoooComposedActivity0(input GoooRecord0d8c19ab5f34a1a9ba068ee5a9c2824812be0131a1901904c711675bb7a57c6a) GoooRecordb84d903fa73b073b6352e4a20b21ea24d7ef186b44c53136421a216a22dbc488 {

	var module = "github.com/kimjooyoon/gooo-go-ports/examples/recorded-catalog-comparison"
	var declared = []string{
		module,
		module + "/internal/kernel",
		module + "/internal/definitionkernel",
		module + "/internal/observationkernel",
		module + "/internal/availabilitykernel",
		module + "/internal/availabilityobservationkernel",
		module + "/internal/snapshotkernel",
		module + "/cmd/compare",
	}
	var missing int64 = 0
	var undeclared int64 = 0
	for _, required := range declared {
		var found = false
		for _, observed := range input.GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314 {
			if observed == required {
				found = true
			}
		}
		if !found {
			missing = missing + 1
		}
	}
	for _, observed := range input.GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314 {
		var found = false
		for _, required := range declared {
			if observed == required {
				found = true
			}
		}
		if !found {
			undeclared = undeclared + 1
		}
	}
	var duplicated = GoooComposedActivity0Call0(GoooRecord17467d60761371b1e076a80d4e9915f496bbc5c8f4a0220816a3e04418edf315{GoooField624ca5e4c9ce3df2bea4a3edd17a5a16c65059db25a432cb0480135296ac478b: input.GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314})
	var status = "OBSERVED_EXACT"
	if missing > 0 && undeclared > 0 {
		status = "MISSING_AND_UNDECLARED"
	} else if missing > 0 {
		status = "MISSING_PACKAGES"
	} else if undeclared > 0 {
		status = "UNDECLARED_PACKAGES"
	} else if duplicated {
		status = "REPEATED_PACKAGE"
	}
	if input.GoooField00b8d5090334be4e64baac634f42cafc0982183f09eb4423590d1df35161f236 == "" {
		status = "ORIGIN_UNOBSERVED"
	}
	if len(declared) == 0 || GoooComposedActivity0Call0(GoooRecord17467d60761371b1e076a80d4e9915f496bbc5c8f4a0220816a3e04418edf315{GoooField624ca5e4c9ce3df2bea4a3edd17a5a16c65059db25a432cb0480135296ac478b: declared}) {
		status = "INVALID_DECLARED_SURFACE"
	}
	return GoooRecordb84d903fa73b073b6352e4a20b21ea24d7ef186b44c53136421a216a22dbc488{
		GoooFielde0569a41080f6be27c724234db35b2e0ddaa768ecffdd83f6a9600dae513054a: status,
		GoooFieldce3661ab6a0d747b227aa8f2d94e352d7a8c877b8560bbbf0978bebca863961d: input.GoooField00b8d5090334be4e64baac634f42cafc0982183f09eb4423590d1df35161f236,
		GoooField66ecd7de70125daddd236872bea8837c5b2ad06921b229e3f870ad06364b15f2: declared,
		GoooField403e560f053ca4b4c83f5b8fa5b2dc06903cdf5a948dbf673c55db79c3c0a966: input.GoooField356ca25ac158a791079b942822e0b07e401d9b084e25fe88fcff3bd8b5653314,
		GoooField6426480fdd193f28b440299b1fd9a0a0305719d7ccf70c5cbb0b0c5bf50edbb2: missing,
		GoooField7f4697caa2416ae41bbaebbc0880025422a474f132aa4676e3c164502cc3f09b: undeclared,
		GoooField35f06f6ed7bda307c2758805af8ea14aaf9d719ccc55e4c953edeaeb68be275d: duplicated,
	}

}
func GoooComposedActivity0Call0(input GoooRecord17467d60761371b1e076a80d4e9915f496bbc5c8f4a0220816a3e04418edf315) bool {

	for _, value := range input.GoooField624ca5e4c9ce3df2bea4a3edd17a5a16c65059db25a432cb0480135296ac478b {
		var count int64 = 0
		for _, other := range input.GoooField624ca5e4c9ce3df2bea4a3edd17a5a16c65059db25a432cb0480135296ac478b {
			if value == other {
				count = count + 1
			}
		}
		if count > 1 {
			return true
		}
	}
	return false

}

//gooo:generated:end id="urn:gooo:runtime-package-surface:inspect" kind="activity"
