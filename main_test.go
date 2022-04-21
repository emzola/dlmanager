package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binaryName string

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		binaryName = "dlmanager.exe"
	} else {
		binaryName = "dlmanager"
	}
	cmd := exec.Command("go", "build", "-o", binaryName)
	err := cmd.Run()
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		err := os.Remove(binaryName)
		if err != nil {
			log.Fatalf("Error removing built binary: %v", err)
		}
	}()
	m.Run()
}

func TestSubCommandInvoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	curDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := path.Join(curDir, binaryName)
	t.Log(binaryPath)

	tests := []struct{
		args []string
		input string
		expectedOutputLines []string
		expectedExitCode int
	}{
		{
			args: []string{},
			expectedOutputLines: []string{},
			expectedExitCode: 1,
		},
		{
			args: []string{"download"},
			expectedOutputLines: []string{"you have to specify the remote server"},
			expectedExitCode: 1,
		},
		{
			args: []string{"download", "127.0.0.1"},
			expectedExitCode: 0,
		},
		{
			args: []string{"download", "-location", "./downloads", "127.0.0.1"},
			expectedExitCode: 0,
		},
		{
			args: []string{"download", "-where", "./downloads", "127.0.0.1"},
			expectedOutputLines: []string{"flag provided but not defined: -where"},
			expectedExitCode: 1,
		},
		{
			args: []string{"download", "-x", "2", "-location", "./downloads", "127.0.0.1", "127.0.0.1"},
			expectedExitCode: 0,
		},
		{
			args: []string{"download", "-x", "2", "127.0.0.1", "127.0.0.1"},
			expectedExitCode: 0,
		},
		{
			args: []string{"download", "-p", "2", "-location", "./downloads", "127.0.0.1", "127.0.0.1"},
			expectedOutputLines: []string{"flag provided but not defined: -p"},
			expectedExitCode: 1,
		},
		{
			args: []string{"download", "-x", "2", "-location", "./downloads", "127.0.0.1"},
			expectedOutputLines: []string{"you have to specify a remote server for each file to download"},
			expectedExitCode: 1,
		},
		{
			args: []string{"download", "-x", "", "-location", "./downloads", "127.0.0.1"},
			expectedOutputLines: []string{"you have to specify the number of files to download"},
			expectedExitCode: 1,
		},
	}

	byteBuf := new(bytes.Buffer)

	for _, tc := range tests {
		t.Logf("Executing %v %v\n", binaryPath, tc.args)
		
		cmd := exec.CommandContext(ctx, binaryPath, tc.args...)
		cmd.Stdout = byteBuf
	
		if len(tc.input) != 0 {
			cmd.Stdin = strings.NewReader(tc.input)
		}

		err := cmd.Run()
		if err != nil && tc.expectedExitCode == 0 {
			t.Fatalf("Expected application to exit without an error. Got: %v", err)
		}
		if cmd.ProcessState.ExitCode() != tc.expectedExitCode {
			t.Log(byteBuf.String())
			t.Fatalf("Expected: %v, Got: %v", tc.expectedExitCode, cmd.ProcessState.ExitCode())
		}

		output := byteBuf.String()
		lines := strings.Split(output, "\n")
		for num := range tc.expectedOutputLines {
			if lines[num] != tc.expectedOutputLines[num] {
				t.Fatalf("Expected: %v, Got: %v", tc.expectedOutputLines[num], lines[num])
			}
		}
		byteBuf.Reset()
	}
}

func TestHandleCommand(t *testing.T) {
	usageMessage := `Usage: Download Manager [download] -h

download: An HTTP sub-command for downloading files

download: <options> server

options: 
  -location string
    	Download location (default "./downloads")
  -x int
    	Number of files to download
`
	tests := []struct{
		args []string
		output string
		err error
	}{
		{
			args: []string{},
			output: "invalid sub-command specified\n" + usageMessage,
			err: ErrInvalidSubCommand,
		},
		{
			args: []string{"foo"},
			output: "invalid sub-command specified\n" + usageMessage,
			err: ErrInvalidSubCommand,
		},
		{
			args: []string{"-h"},
			output: usageMessage,
			err: nil,
		},
		{
			args: []string{"-help"},
			output: usageMessage,
			err: nil,
		},
	}

	byteBuf := new(bytes.Buffer)
	for _, tc := range tests {
		err := handleCommand(byteBuf, tc.args)
		if err != nil && tc.err == nil {
			t.Fatalf("Expected nil error. Got: %v", err)
		}
		if tc.err != nil && err.Error() != tc.err.Error() {
			t.Fatalf("Expected: %v, Got: %v", tc.err, err)
		}
		if len(tc.output) != 0 {
			gotOutput := byteBuf.String()
			if tc.output != gotOutput {
				t.Errorf("Expected: %v, Got: %v", tc.output, gotOutput)
			}
		}
		byteBuf.Reset()
	}
}