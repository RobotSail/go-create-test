package cmd

import (
	"io/ioutil"
	"log"

	"github.com/robotsail/go-create-test/pkg/parse"
	"github.com/spf13/cobra"
)

const (
	FlagFilepathFull     = "filepath"
	FlagFunctionNameFull = "function"
)

func NewGenerateTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-tests",
		Short: "Generate tests for the functions in the provided file",
		RunE:  RunGenerateTests,
	}

	cmd.Flags().StringP(FlagFilepathFull, "f", "", "path to the file containing the functions to be tested")
	cmd.Flags().StringP(FlagFunctionNameFull, "n", "", "name of the function to be tested")

	return cmd
}

type GenerateTestsOptions struct {
	Filepath     string
	FunctionName string
}

func parseGenerateTestsOptions(cmd *cobra.Command) (opts GenerateTestsOptions, err error) {
	opts.Filepath, err = cmd.Flags().GetString(FlagFilepathFull)
	if err != nil {
		return
	}
	opts.FunctionName, err = cmd.Flags().GetString(FlagFunctionNameFull)
	return
}

func RunGenerateTests(cmd *cobra.Command, args []string) error {
	opts, err := parseGenerateTestsOptions(cmd)
	if err != nil {
		return err
	}

	// entry point
	log.Printf("got %q and %q", opts.Filepath, opts.FunctionName)

	code, err := ioutil.ReadFile(opts.Filepath)
	if err != nil {
		return err
	}

	parse.ParseCode(opts.Filepath, opts.FunctionName, code)
	return nil
}
