package main

import (
	"os"
)

var version = "dev"

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
