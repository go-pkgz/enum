// Package main provides entry point for enum generator
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/go-pkgz/enum/internal/generator"
)

// allow mocking os.Exit in tests
var osExit = os.Exit

func main() {
	typeFlag := flag.String("type", "", "type name (must be lowercase)")
	pathFlag := flag.String("path", "", "output directory path (default: same as source)")
	lowerFlag := flag.Bool("lower", false, "use lower case for marshaled/unmarshaled values")
	getterFlag := flag.Bool("getter", false, "generate getter methods for enum values")
	helpFlag := flag.Bool("help", false, "show usage")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Parse()

	// collect build info (version), new in go 1.24
	buildInfo := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" {
			buildInfo = info.Main.Version
		}
	}

	if *helpFlag {
		showUsage()
		osExit(0)
		return
	}
	if *versionFlag {
		fmt.Printf("enum generator %s\n", buildInfo)
		osExit(0)
		return
	}

	gen, err := generator.New(*typeFlag, *pathFlag)
	if err != nil {
		fmt.Printf("%v\n", err)
		showUsage()
		osExit(1)
		return
	}

	gen.SetLowerCase(*lowerFlag)
	gen.SetGenerateGetter(*getterFlag)

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
