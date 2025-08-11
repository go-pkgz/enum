package testdata

// test various underlying types
type uint16Type uint16

const (
	uint16TypeFirst uint16Type = iota
	uint16TypeSecond
)

type int32Type int32

const (
	int32TypeAlpha int32Type = iota + 100
	int32TypeBeta
)

type byteType byte

const (
	byteTypeA byteType = iota
	byteTypeB
)

type runeType rune

const (
	runeTypeX runeType = 'A'
	runeTypeY runeType = 'B'
)
