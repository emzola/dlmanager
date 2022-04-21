# dlmanager
dlmanager is a HTTP client for downloading files from the internet. It has the following features:
- Supports single and multiple file downloads concurrently
- Saves downloaded files to a given location. If the given location path does not exist, it creates all the missing directories in the path. If a location is not specified, it uses the project’s download directory
- Resumes downloads after connection failure
- Keeps track of download progress concurrently
- Shuts down gracefully
- Validates downloads using checksums