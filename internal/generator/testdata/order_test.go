package testdata

type orderTest uint8

const (
	orderTestZero    orderTest = iota // Should be first (alphabetically would be fourth)
	orderTestAlpha                    // Should be second (alphabetically would be first)
	orderTestCharlie                  // Should be third (alphabetically would be third)
	orderTestBravo                    // Should be fourth (alphabetically would be second)
)
