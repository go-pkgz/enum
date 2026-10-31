---
worth: later
where: README.md
added: 2026-10-31
---
# README does not say _test.go files are ignored

`Parse` skips `_test.go` files, so enum constants declared in tests are never collected. The README
describes which constants are picked up but says nothing about test files, so someone declaring an
enum in a test file gets "no const values found" with no hint why. One line in the constants section
covers it.
