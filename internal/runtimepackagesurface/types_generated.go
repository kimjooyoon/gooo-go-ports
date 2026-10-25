// Code generated from the source-bound Gooo typed plan; DO NOT EDIT.
package runtimepackagesurface

type Observation struct {
	OriginRef   string   `json:"origin_ref"`
	ImportPaths []string `json:"import_paths"`
}

type Assessment struct {
	Status               string   `json:"status"`
	ObservationOrigin    string   `json:"observation_origin"`
	DeclaredPaths        []string `json:"declared_paths"`
	ObservedPaths        []string `json:"observed_paths"`
	MissingCount         int64    `json:"missing_count"`
	UndeclaredCount      int64    `json:"undeclared_count"`
	DuplicateObservation bool     `json:"duplicate_observation"`
}
