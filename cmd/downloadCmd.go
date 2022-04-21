package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type downloadConfig struct {
	url      []string
	location string
	numFiles int
}

// httpClient creates an HTTP client
func httpClient() *http.Client {
	// redirectPolicyFunc does not follow redirection request
	redirectPolicyFunc := func(r *http.Request, via []*http.Request) error {
		if len(via) >= 1 {
			return errors.New("stopped after 1 redirect")
		}
		return nil
	}

	// Configure the connection pool
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          25,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		CheckRedirect: redirectPolicyFunc,
		Transport:     t,
	}
}

// validateConfig validates downloadConfig and returns an error if it finds any.
func validateConfig(c *downloadConfig, fs *flag.FlagSet) error {
	switch {
	case c.numFiles < 1:
		return InvalidInputError{ErrInvalidCommand}
	case c.numFiles > 1:
		for i := 0; i < c.numFiles; i++ {
			c.url = append(c.url, fs.Arg(i))
		}
	default:
		c.url = append(c.url, fs.Arg(0))
	}
	return nil
}

// HandleDownload handles the download sub-command.
func HandleDownload(w io.Writer, args []string) error {
	c := downloadConfig{}

	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(w)
	fs.StringVar(&c.location, "location", "./downloads", "Download location")
	fs.IntVar(&c.numFiles, "x", 1, "Number of files to download")
	fs.Usage = func() {
		var usageString = `
download: An HTTP sub-command for downloading files.

download: <options> server`
		fmt.Fprint(w, usageString)
		fmt.Fprintln(w)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "options: ")
		fs.PrintDefaults()
	}

	err := fs.Parse(args)
	if err != nil {
		return FlagParsingError{err}
	}

	// Ensure positional arguments is not less than 1
	if !(fs.NArg() > 0) {
		return InvalidInputError{ErrNoServerSpecified}
	}

	// validate the config
	err = validateConfig(&c, fs)
	if err != nil {
		return err
	}

	httpClient := httpClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.url[0], nil)
	if err != nil {
		return err
	}
	r, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	responseBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, string(responseBody))
	return nil
}
