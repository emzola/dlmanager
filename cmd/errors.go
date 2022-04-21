package cmd

import "errors"

var (
	ErrNoServerSpecified = errors.New("you have to specify a remote server for each file to download")
	ErrInvalidCommand = errors.New("invalid HTTP command specified")
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