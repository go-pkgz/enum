package status

//go:generate go run ../../main.go -type status -lower -sql
//go:generate go run ../../main.go -type jobStatus -lower -getter -sql

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
)

type jobStatus uint8

const (
	jobStatusUnknown jobStatus = iota
	jobStatusActive
	jobStatusInactive
	jobStatusBlocked
)
