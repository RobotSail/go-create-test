package lib

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/robotsail/go-create-test/pkg/types"
	openai "github.com/sashabaranov/go-openai"
)

const systemPrompt = `You are a highly skilled machine that writes Golang tests for a living.
You process requests to write test code for a given function and are provided with all of the definitions that the target function calls.
Given a request to test a function, respond only with the code for the entire test file.
`

const prompt = `
You must write a Golang test for the following function:

` + "```" + `go
package {{.PackageName}}

// rest of the file omitted for brevity

{{.TargetFunction}}
` + "```" + `

For context, here are definitions for all of the symbols referenced by the target functions. Use these definitions 
to properly test for any edge cases or fail points.

` + "```" + `go
{{.CalledFunctions}}
` + "```" + `

`

func createTestPrompt(params types.TestCodePrompt) (string, error) {
	tmpl := template.Must(template.New("prompt").Parse(prompt))

	// Execute the template with the given data
	var output strings.Builder
	err := tmpl.Execute(&output, params)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	// Return the formatted data as a string
	return output.String(), nil
}

func GenerateTestCode(params types.TestCodePrompt) (string, error) {
	prompt, err := createTestPrompt(params)
	if err != nil {
		return "", fmt.Errorf("failed to create prompt: %w", err)
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)

	res, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   1024,
		Temperature: 0.1,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}
	// extract response
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}
	generation := res.Choices[0]
	return generation.Message.Content, nil
}

// UnwrapResponse accepts a piece of code which is enclosed within two backtick blocks, like '```\nfoo\n```'.
func UnwrapResponse(response string) string {
	// check if the response is wrapped in backticks
	if !strings.HasPrefix(response, "```go") && !strings.HasSuffix(response, "```") {
		return response
	}

	unwrapped := strings.TrimPrefix(response, "```go")
	unwrapped = strings.TrimSuffix(unwrapped, "```")
	return unwrapped
}
