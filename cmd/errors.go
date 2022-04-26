package cmd

import "errors"

var (
	ErrNoServerSpecified = errors.New("you have to specify a remote server for each file to download")
	ErrNumDownloadFiles = errors.New("you have to specify a number greater than 0 for -x")
	ErrInvalidCommand = errors.New("invalid download command specified")
	ErrNumFilesMustBeZero = errors.New("you have to specify 0 for -x")
)

type InvalidInputError struct {
	Err error
}

func (e InvalidInputError) Error() string {
	return e.Err.Error()
}

type FlagParsingError struct {
	Err error
}

func (e FlagParsingError) Error() string {
	return e.Err.Error()
}