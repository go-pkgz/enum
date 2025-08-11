package testdata

// test multiplication and division
type mulDivType uint8

const (
	mulDivTypeA mulDivType = iota * 2 // 0
	mulDivTypeB                       // 2
	mulDivTypeC                       // 4
)

// test right-side iota
type rightIotaType uint8

const (
	rightIotaTypeX rightIotaType = 10 + iota // 10
	rightIotaTypeY                           // 11
)

// test subtraction
type subType int

const (
	subTypeA subType = 100 - iota // 100
	subTypeB                      // 99
	subTypeC                      // 98
)
