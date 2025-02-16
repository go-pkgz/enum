// Package main provides entry point for enum generator
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-pkgz/enum/internal/generator"
	"runtime/debug"
)

var version = "dev"

// allow mocking os.Exit in tests
var osExit = os.Exit

func main() {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "(devel)" && bi.Main.Version != "" {
		version = bi.Main.Version
	}

	typeFlag := flag.String("type", "", "type name (must be lowercase)")
	pathFlag := flag.String("path", "", "output directory path (default: same as source)")
	lowerFlag := flag.Bool("lower", false, "use lower case for marshaled/unmarshaled values")
	helpFlag := flag.Bool("help", false, "show usage")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Parse()

	if *helpFlag {
		showUsage()
		osExit(0)
		return
	}
	if *versionFlag {
		fmt.Printf("enum generator %s\n", version)
		osExit(0)
		return
	}

	gen, err := generator.New(version, *typeFlag, *pathFlag)
	if err != nil {
		fmt.Printf("%v\n", err)
		showUsage()
		osExit(1)
		return
	}

	gen.SetLowerCase(*lowerFlag)

	if err := gen.Parse("."); err != nil {
		fmt.Printf("%v\n", err)
		osExit(1)
		return
	}

	if err := gen.Generate(); err != nil {
		fmt.Printf("%v\n", err)
		osExit(1)
		return
	}
}

func showUsage() {
	fmt.Printf("usage: enumgen -type <type> [-path <path>] [-lower] [-version]\n")
	fmt.Printf("  -type <type>    type name (must be lowercase)\n")
	fmt.Printf("  -path <path>    output directory path (default: same as source)\n")
	fmt.Printf("  -lower          use lower case for marshaled/unmarshaled values\n")
	fmt.Printf("  -version        print version\n")
}
