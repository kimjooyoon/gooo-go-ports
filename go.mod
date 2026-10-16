module github.com/kimjooyoon/gooo-go-ports

go 1.27.2

require (
	github.com/google/uuid v1.6.0
	golang.org/x/net v0.59.0
)

require golang.org/x/text v0.42.0 // indirect

replace github.com/google/uuid => ./third_party/google-uuid
