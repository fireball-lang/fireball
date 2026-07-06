package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type runtimeError struct {
	err error
}

func (e *runtimeError) Error() string {
	return e.err.Error()
}

func (e *runtimeError) Unwrap() error {
	return e.err
}

func runE(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			return &runtimeError{err: err}
		}

		return nil
	}
}

func printError(err error) {
	_, _ = color.New(color.FgRed, color.Bold).Print("error")
	_, _ = color.New(color.Bold).Printf(": %s\n", err.Error())
}

func printUsageError(cmd *cobra.Command, err error) {
	printError(err)

	fmt.Println()
	fmt.Println(cmd.UsageString())
}
