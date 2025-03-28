package testdata

type jobStatus uint8

const (
	jobStatusUnknown jobStatus = iota
	jobStatusActive
	jobStatusInactive
	jobStatusBlocked
)
