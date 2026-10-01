package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/bk3/qwe/internal/qwe"
)

var version = "dev"

func main() {
	if err := qwe.Run(os.Args[1:], version); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				signal.Reset(status.Signal())
				if err := syscall.Kill(os.Getpid(), status.Signal()); err == nil {
					// Allow the default signal handler to terminate this process.
					time.Sleep(time.Second)
				}
				os.Exit(128 + int(status.Signal()))
			}
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "qwe:", err)
		os.Exit(1)
	}
}
