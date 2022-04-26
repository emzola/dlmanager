# dlmanager

dlmanager is an HTTP client for downloading files from the internet. It has the following features:

- Supports single and multiple file downloads concurrently
- Supports downloading urls from file
- Saves downloaded files to a given location (directory). If the given directory path does not exist, it creates all the missing directories in the path. If a directory is not specified, it uses the project’s download directory
- Resumable downloads after connection failure
- Keeps track of download progress concurrently

## Usage

### Basic download

```go
download "https://www.openmymind.net/assets/go/go.pdf"

// Output:
// Downloading https://www.openmymind.net/assets/go/go.pdf...
//    transferred 24576 / 247399 bytes (6.62%)
//    transferred 73728 / 247399 bytes (26.49%)
//    transferred 122880 / 247399 bytes (46.36%)
//    transferred 172032 / 247399 bytes (66.23%)
//    transferred 229376 / 247399 bytes (89.40%)
// File(s) downloaded to ./downloads
```

### Single download

```go
download -location /path/to/dir https://www.openmymind.net/assets/go/go.pdf
```

### Multiple downloads

```go
download -x 2 -location /path/to/dir https://www.openmymind.net/assets/go/go.pdf http://www.golang-book.com/public/pdf/gobook.pdf
```

### Download from file containing list of urls

```go
download -location /path/to/dir -url-file /path/to/file
```
