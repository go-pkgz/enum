---
worth: maybe
where: internal/generator/generator.go:Parse
added: 2026-10-31
---
# build-ignored file of another package makes the package clause random

`Parse` reads every non-test `.go` file in the directory without evaluating build constraints, groups
them by package and keeps whichever package the map visits last. A `//go:build ignore` helper of
`package main` next to the enum (a common pattern for generator scripts) gives the generated file
`package main` in some runs: 2 of 6 in a probe after the `_test.go` skip landed.

Two shapes are open: honour build constraints (`go/build` or `build.Default.MatchFile`), or pick the
package the enum type is declared in instead of the last one seen. Surfaced reviewing
https://github.com/go-pkgz/enum/discussions/11.
