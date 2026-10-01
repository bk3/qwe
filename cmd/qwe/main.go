package main

import (
	"fmt"
	"os"

	"github.com/bk3/qwe/internal/qwe"
)

var version = "dev"

func main() {
	if err := qwe.Run(os.Args[1:], version); err != nil {
		fmt.Fprintln(os.Stderr, "qwe:", err)
		os.Exit(1)
	}
}
