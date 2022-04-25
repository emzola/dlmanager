package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
		err = cmd.InvalidInputError{Err: ErrInvalidSubCommand}
	} else {
		switch args[0] {
		case "-h":
			err = printUsage(w)
		case "-help":
			err = printUsage(w)
		case "download":
			err = cmd.HandleDownload(w, args[1:])
		default:
			err = cmd.InvalidInputError{Err: ErrInvalidSubCommand}
		}
	}
	if err != nil {
		if !errors.As(err, &cmd.FlagParsingError{}) {
			fmt.Fprintln(w, err.Error())
		}
		if errors.As(err, &cmd.InvalidInputError{}) {
			printUsage(w)
		}
	}
	return err
}

// setupSignalHandler reads operating system signals.
func setupSignalHandler(w io.Writer, cancelFunc context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-c
		fmt.Fprintf(w, "Got signal: %v\n", s)
		cancelFunc()
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go setupSignalHandler(os.Stdout, cancel)
	
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			default:
				err := handleCommand(os.Stdout, os.Args[1:])
				if err != nil {
					os.Exit(1)
				}
			}
		}
	}()
	wg.Wait()
}
