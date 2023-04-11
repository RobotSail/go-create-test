package main

import (
	"fmt"
	"os"

	"github.com/robotsail/go-create-test/pkg/cmd"
)

var requiredEnvs []string = []string{"OPENAI_API_KEY"}

func checkEnv() {
	for _, env := range requiredEnvs {
		if _, ok := os.LookupEnv(env); !ok {
			fmt.Printf("Missing required environment variable: %s", env)
			os.Exit(1)
		}
	}
}

func main() {
	checkEnv()
	rootCmd := cmd.NewGenerateTestCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
