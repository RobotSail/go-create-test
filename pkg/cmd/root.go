package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	var filepath string
	var functionName string

	rootCmd := &cobra.Command{
		Use:   "create-test",
		Short: "Generate a test for a given function within the provided file",
		Run: func(cmd *cobra.Command, args []string) {
			Test(filepath, functionName)
		},
	}
	rootCmd.AddCommand(NewGenerateTestCmd())
	return rootCmd
}

func Test(filename string, functionName string) {
	log.Println(`got "` + filename + `" and "` + functionName + `"`)
}
