package starlark

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorSource     ErrorKind = "source"
	ErrorCompile    ErrorKind = "compile"
	ErrorLoad       ErrorKind = "load"
	ErrorInitialize ErrorKind = "initialize"
	ErrorSetup      ErrorKind = "setup"
	ErrorValidation ErrorKind = "validation"
	ErrorInvocation ErrorKind = "invocation"
	ErrorStepLimit  ErrorKind = "step_limit"
	ErrorDeadline   ErrorKind = "deadline"
	ErrorCanceled   ErrorKind = "canceled"
	ErrorStale      ErrorKind = "stale_generation"
	ErrorResult     ErrorKind = "invalid_result"
)

type RuntimeError struct {
	Kind   ErrorKind
	Phase  string
	Source string
	Err    error
}

func (runtimeError *RuntimeError) Error() string {
	if runtimeError == nil {
		return "<nil>"
	}
	prefix := string(runtimeError.Kind)
	if runtimeError.Phase != "" {
		prefix += " " + runtimeError.Phase
	}
	if runtimeError.Source != "" {
		prefix += " " + runtimeError.Source
	}
	if runtimeError.Err != nil {
		return prefix + ": " + runtimeError.Err.Error()
	}
	return prefix
}

func (runtimeError *RuntimeError) Unwrap() error {
	if runtimeError == nil {
		return nil
	}
	return runtimeError.Err
}

func newRuntimeError(kind ErrorKind, phase, source string, err error) error {
	if err == nil {
		err = errors.New("operation failed")
	}
	return &RuntimeError{Kind: kind, Phase: phase, Source: source, Err: err}
}

func IsErrorKind(err error, kind ErrorKind) bool {
	var runtimeError *RuntimeError
	return errors.As(err, &runtimeError) && runtimeError.Kind == kind
}

func sanitizeEvaluationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("Starlark evaluation failed: %s", err.Error())
}
