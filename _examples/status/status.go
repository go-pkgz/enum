package status

//go:generate go run github.com/go-pkgz/enum@master -type status -lower

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
)
