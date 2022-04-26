package cmd

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
)

type downloadConfig struct {
	url      []string
	location string
	numFiles int
	mu       *sync.Mutex
}

// validateConfig validates downloadConfig and returns an error if it finds any.
func validateConfig(file string, config *downloadConfig, fs *flag.FlagSet) error {
	var isFile bool
	if len(file) != 0 {
		isFile = true
	}

	// guard against -x option and -url-file options both specified
	if isFile && config.numFiles != 0 {
		return InvalidInputError{ErrNumFilesMustBeZero}
	}

	// guard against specifying 0 or a negative number for -x option
	// if -url-file option is not specified
	if !isFile && config.numFiles == 0 {
		config.numFiles = 1
	}

	if !isFile && config.numFiles < 0 {
		return InvalidInputError{ErrNumDownloadFiles}
	}

	// validate positional arguments
	if !(fs.NArg() > 0) && !isFile {
		return InvalidInputError{ErrNoServerSpecified}
	}
	if !isFile && fs.NArg() != config.numFiles {
		return InvalidInputError{ErrNoServerSpecified}
	}

	return nil
}

// setDownloadLocation sets the download location of the file.
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
		return fileSize, err
	}
	if f.IsDir() {
		return fileSize, err
	}
	fileSize = f.Size()
	return fileSize, nil
}

// writeToDestinationFile writes data to destination file.
func writeToDestinationFile(filepath string, r *http.Response, bytesChan chan int64) error {
	fInfo, err := getExistingFileSize(filepath)
	if err != nil {
		return err
	}

	// Set flag based on the existence of file in download destination
	flag := os.O_CREATE | os.O_WRONLY
	if fInfo > 0 {
		flag = os.O_APPEND | os.O_WRONLY
	}

	file, err := os.OpenFile(filepath, flag, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	// Move to the end of the file if some data is already downloaded into file
	whence := io.SeekStart
	if fInfo > 0 {
		whence = io.SeekEnd
	}
	_, err = file.Seek(0, whence)
	if err != nil {
		return err
	}

	mu := sync.Mutex{}
	chunkSize := 32 * 1024
	bytes := make([]byte, chunkSize)
	var written int64

	for {
		// Populate the bytes slice
		bytesRead, err := r.Body.Read(bytes)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		if bytesRead > 0 {
			// Write the data from the bytes slice to destination file
			fw, err := file.Write(bytes[0:bytesRead])
			if err != nil {
				return err
			}
			if fw > 0 {
				mu.Lock()
				written += int64(fw)
				mu.Unlock()
				bytesChan <- written
			}
		}
	}
	return nil
}

// getContentLength returns an int64 of the Content-Length of each single file to be downloaded.
func getContentLength(client *http.Client, url string) (int64, error) {
	resp, err := sendHTTPHeadRequest(url, client)
	if err != nil {
		return 0, err
	}
	return resp.ContentLength, nil
}

// getTotalContentLength returns int64 of the total Content-Length of all files to be downloaded.
// The total content length returned is used to calculate the download progress percentage.
func getTotalContentLength(client *http.Client, config *downloadConfig) (int64, error) {
	var contentLength int64
	for _, u := range config.url {
		resp, err := sendHTTPHeadRequest(u, client)
		if err != nil {
			return contentLength, err
		}
		contentLength += resp.ContentLength
	}
	return contentLength, nil
}

// calculateDownloadPercentage returns a float64 of the total download percentage.
func calculateDownloadPercentage(bytes, contentLength int64) float64 {
	x := float64(bytes) / 1e+6
	y := float64(contentLength) / 1e+6
	return (x / y) * 100
}

// displayDownloadInfo shows download progress info to the output stream.
func displayDownloadInfo(w io.Writer, contentLength int64, bytes chan int64, err chan error) {
	for {
		select {
		case <-bytes:
			// TODO: FIX downloadPercentage to not calculate wrongly when showing progress
			downloadPercentage := calculateDownloadPercentage(<-bytes, contentLength)
			fmt.Fprintf(w, "\ttransferred %d / %d bytes (%.2f%%)\n", <-bytes, contentLength, downloadPercentage)
		case <-err:
			func() error {
				return <-err
			}()
		}
	}
}

// readUrlFromFile reads a list of urls from a file.
func readUrlFromFile(file string, config *downloadConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		config.url = append(config.url, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// HandleDownload handles the download sub-command.
func HandleDownload(w io.Writer, args []string) error {
	var urlFile string
	c := &downloadConfig{}
	c.mu = new(sync.Mutex)

	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(w)
	fs.StringVar(&c.location, "location", "./downloads", "Download location")
	fs.IntVar(&c.numFiles, "x", 0, "Number of files to download")
	fs.StringVar(&urlFile, "url-file", "", "File containing list of url")
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

	// Validate the config
	err = validateConfig(urlFile, c, fs)
	if err != nil {
		return err
	}

	// Read from file if -url-file flag is provided,
	// otherwise read urls from positional args specified
	if len(urlFile) != 0 {
		err := readUrlFromFile(urlFile, c)
		if err != nil {
			return err
		}
	} else {
		switch {
		case c.numFiles > 1:
			for i := 0; i < c.numFiles; i++ {
				c.url = append(c.url, fs.Arg(i))
			}
		case c.numFiles == 1:
			c.url = append(c.url, fs.Arg(0))			
		}	
	}

	httpClient := httpClient()

	bytesChan := make(chan int64)
	errorChan := make(chan error)

	// Get the Content-Length of all files to download
	totalContentLength, err := getTotalContentLength(httpClient, c)
	if err != nil {
		return err
	}

	// Display download progress info
	go displayDownloadInfo(w, totalContentLength, bytesChan, errorChan)

	var wg sync.WaitGroup
	for _, u := range c.url {
		fmt.Fprintf(w, "Downloading %v...\n", u)
		wg.Add(1)
		go func(url string, config *downloadConfig) {
			defer wg.Done()
			c.mu.Lock()
			defer c.mu.Unlock()

			// Get filename before download
			r, err := sendHTTPRequest(url, httpClient)
			if err != nil {
				errorChan <- err
			}
			defer r.Body.Close()
			filename, err := getFileName(r)
			if err != nil {
				errorChan <- err
			}

			// Set download destination
			setDownloadLocation, err := setDownloadLocation(c.location)
			if err != nil {
				errorChan <- err
			}
			destinationPath := filepath.Join(setDownloadLocation, filename)

			// Get file size from download destination
			existingFileSize, err := getExistingFileSize(destinationPath)
			if err != nil {
				errorChan <- err
			}

			// Get the content length of each file
			contentLength, err := getContentLength(httpClient, url)
			if err != nil {
				errorChan <- err
			}

			// Compare the content length of each file with an existing file size. If they are equal,
			// no need to download file because has already downloaded completely.
			if existingFileSize == contentLength {
				return
			}

			// Make the HTTP request to download file
			resp, err := sendHTTPRequestWithHeader(url, httpClient, existingFileSize)
			if err != nil {
				errorChan <- err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
				errorChan <- fmt.Errorf("unexpected Status Code: %v", resp.StatusCode)
			}

			// Write to destination file
			err = writeToDestinationFile(destinationPath, resp, bytesChan)
			if err != nil {
				errorChan <- err
			}
		}(u, c)
	}
	wg.Wait()
	fmt.Fprintf(w, "File(s) downloaded to %s\n", c.location)
	return nil
}
