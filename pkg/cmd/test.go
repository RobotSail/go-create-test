package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/briandowns/spinner"
	"github.com/robotsail/go-create-test/pkg/lib"
	"github.com/robotsail/go-create-test/pkg/parse"
	"github.com/robotsail/go-create-test/pkg/types"
	"github.com/spf13/cobra"
)

const (
	FlagFilepathFull     = "filepath"
	FlagFunctionNameFull = "function"
	FlagProjectDirectory = "dir"
)

func NewGenerateTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-tests",
		Short: "Generate tests for the functions in the provided file",
		RunE:  RunGenerateTests,
	}

	cmd.Flags().StringP(FlagFilepathFull, "f", "", "path to the file containing the functions to be tested")
	cmd.Flags().StringP(FlagFunctionNameFull, "n", "", "name of the function to be tested")
	cmd.Flags().StringP(FlagProjectDirectory, "d", "", "path to the project directory (optional)")
	requiredFlags := []string{FlagFilepathFull, FlagFunctionNameFull, FlagProjectDirectory}
	for _, flag := range requiredFlags {
		err := cmd.MarkFlagRequired(flag)
		if err != nil {
			log.Fatalf("error marking flag as required: %v", err)
		}
	}

	return cmd
}

type GenerateTestsOptions struct {
	Filepath     string
	FunctionName string
	ProjectDir   string
}

func parseGenerateTestsOptions(cmd *cobra.Command) (opts GenerateTestsOptions, err error) {
	opts.Filepath, err = cmd.Flags().GetString(FlagFilepathFull)
	if err != nil {
		return
	}
	opts.FunctionName, err = cmd.Flags().GetString(FlagFunctionNameFull)
	if err != nil {
		return
	}
	opts.ProjectDir, err = cmd.Flags().GetString(FlagProjectDirectory)
	return
}

func RunGenerateTests(cmd *cobra.Command, args []string) error {
	opts, err := parseGenerateTestsOptions(cmd)
	if err != nil {
		return err
	}

	err = os.Chdir(opts.ProjectDir)
	if err != nil {
		fmt.Printf("error changing directories: %v\n", err)
		return err
	}
	// entry point
	log.Printf("got %q and %q", opts.Filepath, opts.FunctionName)

	code, err := ioutil.ReadFile(opts.Filepath)
	if err != nil {
		return err
	}

	packageName, err := parse.GetPackageName(code)
	if err != nil {
		return err
	}
	log.Printf("packageName: %q\n", packageName)

	funcDef, err := parse.GetFunctionDefinition(opts.FunctionName, code)
	if err != nil {
		return err
	}

	callDefs, err := parse.GetFunctionCalls(opts.Filepath, opts.FunctionName, code)
	if err != nil {
		return err
	}

	testFileName := lib.GetTestFileName(opts.Filepath)
	s := spinner.New(spinner.CharSets[20], 100*time.Millisecond) // Build our new spinner
	s.Prefix = "Generating test code... "
	s.FinalMSG = fmt.Sprintf("Done! Test file written to %s\n", testFileName)
	s.Start() // Start the spinner
	testFile, err := lib.GenerateTestCode(types.TestCodePrompt{
		TargetFunction:  funcDef,
		CalledFunctions: callDefs,
		PackageName:     packageName,
	})
	s.Stop()
	if err != nil {
		return fmt.Errorf("error generating test code: %w", err)
	}

	sanitizedResponse := lib.UnwrapResponse(testFile)
	fmt.Printf("")
	err = ioutil.WriteFile(path.Join(path.Dir(opts.Filepath), testFileName), []byte(sanitizedResponse), 0644)
	if err != nil {
		return fmt.Errorf("error writing test file: %w", err)
	}
	return nil
}
