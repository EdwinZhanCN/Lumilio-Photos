module github.com/riverqueue/river

go 1.23.0

require (
	github.com/riverqueue/river/riverdriver v0.24.0
	github.com/riverqueue/river/rivershared v0.24.0
	github.com/riverqueue/river/rivertype v0.24.0
	github.com/tidwall/gjson v1.18.0
	github.com/tidwall/sjson v1.2.5
	golang.org/x/sync v0.16.0
)

replace github.com/riverqueue/river/rivershared => ../../../third_party/river-rivershared
