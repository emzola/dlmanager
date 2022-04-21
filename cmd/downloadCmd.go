package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
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

// downloadLocation sets the download location of the file.
// If the given file path does not exist, it creates all the missing directories in the path. 
func downloadLocation(location string) (string, error) {
	_, err := os.Stat(location)
	if err != nil {	
		if !errors.Is(err, fs.ErrNotExist) {
			return "", errors.New("error checking download directory" + err.Error())
		}
		locationPath := filepath.FromSlash(location)
		err := os.MkdirAll(locationPath, 0777)
		if err != nil {
			return "", errors.New("error creating download directory" + err.Error())
		}		
	}
	return location, nil
}

// getFileName fetches the name of the downloadable file.
func getFileName(r *http.Response) (string, error) {
	filename := r.Request.URL.Path
	contentDisposition := r.Header.Get("Content-Disposition")
	if len(contentDisposition) != 0 {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			val, ok := params["filename"]
			if ok {
				filename = val
			}
		}
	}
	filename = filepath.Base(path.Clean("/" + filename))
	if len(filename) == 0 || filename == "." || filename == "/" {
		return "", errors.New("filename couldn't be determined")
	}
	return filename, nil
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
download: An HTTP sub-command for downloading files

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

	// Get filename before download
	filename, err := getFileName(r)
	if err != nil {
		return err
	}

	// Get download location
	downloadLocation, err := downloadLocation(c.location)
	if err != nil {
		return err
	}

	responseBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Change into the download location directory
	err = os.Chdir(downloadLocation)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(responseBody)
	if err != nil {
		return err
	}	

	fmt.Fprintf(w, "Data saved to %s\n", filename)
	return nil
}
