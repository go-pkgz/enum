package testdata

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
)
