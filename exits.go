package main

import (
	"fmt"
	"os"
)

type ExitCode int

const (
	ExitOk    ExitCode = 0
	ExitFlags ExitCode = 1
	ExitOther ExitCode = 100
)

type ExitError struct {
	err  error
	exit ExitCode
}

func (e ExitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e ExitError) UnWrap() error {
	return e.err
}

func Exit(code ExitCode) {
	os.Exit(int(code))
}

func Fatalf(code ExitCode, msg string, args ...interface{}) error {
	return ExitError{
		err:  fmt.Errorf(msg, args...),
		exit: code,
	}
}
