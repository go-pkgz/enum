package testdata

type repeatValues uint8

const (
	repeatValuesFirst  repeatValues = 10
	repeatValuesSecond repeatValues // This should repeat the value 10
	repeatValuesThird  repeatValues = 20
	repeatValuesFourth repeatValues // This should repeat the value 20
)
