package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/emzola/dlmanager/cmd"
)

var ErrInvalidSubCommand = errors.New("invalid sub-command specified")

// printUsage displays help information.
func printUsage(w io.Writer) error {
	fmt.Fprintln(w, "Usage: Download Manager [download] -h")
	cmd.HandleDownload(w, []string{"-h"})
	return nil
}

// handleCommand determines which sub-command to execute based on user input.
func handleCommand(w io.Writer, args []string) error {
	var err error

	if len(args) < 1 {
		err = ErrInvalidSubCommand
	} else {
		switch args[0] {
		case "-h":
			err = printUsage(w)
		case "-help":
			err = printUsage(w)
		case "download":
			err = cmd.HandleDownload(w, args[1:])
		default:
			err = ErrInvalidSubCommand
		}
	}
	if err != nil {
		fmt.Fprintln(w, err.Error())
		printUsage(w)
	}
	return err
}

func main() {
	err := handleCommand(os.Stdout, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}