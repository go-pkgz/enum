package integration

//go:generate ../../../../enum -type=priority -sql -bson -yaml

type priority int32

const (
	priorityNone     priority = -1
	priorityLow      priority = 0
	priorityMedium   priority = 100
	priorityHigh     priority = 1000
	priorityCritical priority = 999999
)
