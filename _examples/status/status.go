package status

//go:generate go run ../../main.go -type status -lower
//go:generate go run ../../main.go -type jobStatus -lower

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
