package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func startTestHTTPServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new-url", http.StatusMovedPermanently)
	}) 
	return httptest.NewServer(mux) 
}

func TestHandleDownload(t *testing.T) {
	usageMessage := `
download: An HTTP sub-command for downloading files

download: <options> server

options: 
  -location string
    	Download location (default "./downloads")
  -x int
    	Number of files to download
`
	ts := startTestHTTPServer()
	defer ts.Close()

	tests := []struct{
		args []string
		output string
		err error
	}{
		{
			args: []string{},
			err: ErrNoServerSpecified,
		},
		{
			args: []string{"-h"},
			output: usageMessage,
			err: errors.New("flag: help requested"),
		},
		{
			args: []string{ts.URL + "/redirect"},
			err: errors.New(`Get "/new-url": stopped after 1 redirect`),
		},
	}

	byteBuf := new(bytes.Buffer)
	for _, tc := range tests {
		err := HandleDownload(byteBuf, tc.args)
		if err != nil && tc.err == nil {
			t.Fatalf("Expected nil error. Got: %v", err)
		}
		if tc.err != nil && err == nil {
			t.Fatalf("Expected non-nil error, Got: %v", err)
		}
		if tc.err != nil && tc.err.Error() != err.Error() {
			t.Fatalf("Expected: %v, Got: %v", tc.err, err)
		}
		if len(tc.output) != 0 {
			gotOutput := byteBuf.String()
			if tc.output != gotOutput {
				t.Errorf("Expected: %s, Got: %s", tc.output, gotOutput)
			}
		}
		byteBuf.Reset()
	}
}