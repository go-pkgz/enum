package integration

//go:generate ../../../../enum -type=status -lower -sql -bson -yaml

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
	statusDeleted
	statusPending
	statusArchived
)
