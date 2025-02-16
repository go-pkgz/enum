package status

//go:generate go run ../../main.go -type status -lower

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
)
