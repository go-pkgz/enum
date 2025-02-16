// Package main provides command line tool to generate enum code from the type definition.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-pkgz/enum/internal/generator"
)

var version = "dev"

func main() {
	typeFlag := flag.String("type", "", "type name (must be lowercase)")
	pathFlag := flag.String("path", "", "output directory path (default: same as source)")
	lowerFlag := flag.Bool("lower", false, "use lower case for marshaled/unmarshaled values")
	helpFlag := flag.Bool("help", false, "show usage")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Parse()

	if *helpFlag {
		showUsage()
		os.Exit(0)
	}
	if *versionFlag {
		fmt.Printf("enum generator %s\n", version)
		os.Exit(0)
	}

	gen, err := generator.New(*typeFlag, *pathFlag)
	if err != nil {
		fmt.Printf("%v\n", err)
		showUsage()
		os.Exit(1)
	}

	gen.SetLowerCase(*lowerFlag)

	if err := gen.Parse("."); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if err := gen.Generate(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Printf("usage: enumgen -type <type> [-path <path>] [-lower] [-version]\n")
	fmt.Printf("  -type <type>    type name (must be lowercase)\n")
	fmt.Printf("  -path <path>    output directory path (default: same as source)\n")
	fmt.Printf("  -lower          use lower case for marshaled/unmarshaled values\n")
	fmt.Printf("  -version        print version\n")
}
