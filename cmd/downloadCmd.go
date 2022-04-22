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
func setDownloadLocation(location string) (string, error) {
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

// getExistingFileSize checks for the existence of the file in the download destination directory.
// If the file already exists, it returns an integer > 0. If the file does not exist, it returns 0.
func getExistingFileSize(filename string) (int64, error) {
	var fileSize int64
	f, err := os.Stat(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fileSize, nil
		}
	} else {
		return fileSize, err
	}
	if f.IsDir() {
		return fileSize, err
	}
	fileSize = f.Size()
	return fileSize, nil
}

// httpClient creates an HTTP client.
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

// sendHTTPRequest sends an HTTP request and returns a response.
func sendHTTPRequest(c downloadConfig, client *http.Client) (*http.Response, error){
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.url[0], nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// sendHTTPRequestWithHeader sends an HTTP request with range header and returns a response.
func sendHTTPRequestWithHeader(c downloadConfig, client *http.Client, fileSize int64) (*http.Response, error){
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.url[0], nil)
	if err != nil {
		return nil, err
	}

	// Set range header to the request if file already exists at destination path
	if fileSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%v-", fileSize))
	}	
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// writeToDestinationFile writes data to destination file.
func writeToDestinationFile(filepath string, r *http.Response) error {
	// Set flag based on the existence of file
	flag := os.O_CREATE | os.O_WRONLY
	fInfo, err := getExistingFileSize(filepath)
	if err == nil {
		if fInfo > 0 {
			flag = os.O_APPEND | os.O_WRONLY
		} 
	}	

	fWriter, err := os.OpenFile(filepath, flag, 0666)
	if err != nil {
		return err
	}

	// Move to the end of the file if some data is already downloaded
	whence := io.SeekStart
	if fInfo > 0 {
		whence = io.SeekEnd
	}
	_, err = fWriter.Seek(0, whence)
	if err != nil {
		return err
	}

	byteBuf := make([]byte, 32 * 1024)
	var written int64
	for {
		// Read data from response body to buffer
		nr, er := r.Body.Read(byteBuf)
		if nr > 0 {
			// Write the buffer to destination file
			nw, ew := fWriter.Write(byteBuf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return ew
			}
			if nr != nw {
				return io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return er
		}
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

	// Get filename before download
	r, err := sendHTTPRequest(c, httpClient)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	filename, err := getFileName(r)
	if err != nil {
		return err
	}

	// Set download destination
	setDownloadLocation, err := setDownloadLocation(c.location)
	if err != nil {
		return err
	}
	destinationPath := filepath.Join(setDownloadLocation, filename)

	// Get file size from download destination
	existingFileSize, err := getExistingFileSize(destinationPath)
	if err != nil {
		return err
	}
	
	resp, err := sendHTTPRequestWithHeader(c, httpClient, existingFileSize)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected Status Code: %v", resp.StatusCode)
	}	

	// write to destination file
	err = writeToDestinationFile(destinationPath, resp)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Data saved to %s\n", filename)
	return nil
}
