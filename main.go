package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

var testGoFile = path.Join("example", "test.go")

// LoadTestFile reads in the test file and returns the contents as a string
func LoadTestFile() ([]byte, error) {
	// load the test file from testGoFile
	data, err := ioutil.ReadFile(testGoFile)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type FunctionDeclaration struct {
	Name string
	Body string
}

type FunctionCall struct {
	Name     string
	Location sitter.Point
}

func main() {
	fileContents, err := LoadTestFile()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = parseCode(fileContents)
	if err != nil {
		fmt.Printf("Error parsing source code: %v", err)
		os.Exit(1)
	}
}

func parseCode(code []byte) error {
	log.Println("parsing code")
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, code)
	if err != nil {
		return err
	}
	functions := extractFunctions(tree.RootNode(), code)
	for _, function := range functions {
		fmt.Printf("Found function '%s', definition:\n%s\n\n", function.Name, function.Body)
	}

	functionCalls := findFunctionCalls(tree.RootNode(), code)
	for _, call := range functionCalls {
		fmt.Printf("Found function call '%s' at '%d:%d'\n", call.Name, call.Location.Row, call.Location.Column)
	}

	err = findDefinitions(functionCalls)
	if err != nil {
		return fmt.Errorf("error finding definitions: %v", err)
	}
	tree.Close()
	return nil
}

func extractFunctions(t *sitter.Node, source []byte) []FunctionDeclaration {
	functions := []FunctionDeclaration{}
	if t.Type() == "function_declaration" {
		functions = append(functions, FunctionDeclaration{
			Body: string(t.Content([]byte(source))),
			Name: string(t.Child(1).Content([]byte(source))),
		},
		)
	}
	for i := 0; i < int(t.ChildCount()); i++ {
		childNode := t.Child(i)
		functions = append(functions, extractFunctions(childNode, source)...)
	}
	return functions
}

// findFunctionCalls Returns a list of function calls in the source code
func findFunctionCalls(t *sitter.Node, source []byte) []FunctionCall {
	calls := []FunctionCall{}
	if t.Type() == "call_expression" {
		calls = append(calls, FunctionCall{
			Name:     string(t.Child(0).Content([]byte(source))),
			Location: t.StartPoint(),
		})
	}
	for i := 0; i < int(t.ChildCount()); i++ {
		childNode := t.Child(i)
		calls = append(calls, findFunctionCalls(childNode, source)...)
	}
	return calls
}

func findDefinitions(calls []FunctionCall) error {
	for _, call := range calls {
		out, err := exec.Command("gopls", "definition", fmt.Sprintf("%s:%d:%d", testGoFile, call.Location.Row+1, call.Location.Column+1)).Output()
		if err != nil {
			return err
		}
		fmt.Printf("Definition for '%s' is at '%s'\n", call.Name, string(out))
	}
	return nil
}

// import (
// 	"context"
// 	"fmt"

// 	sitter "github.com/smacker/go-tree-sitter"
// 	"github.com/smacker/go-tree-sitter/golang"
// )

// func main() {
// 	parser := sitter.NewParser()
// 	parser.SetLanguage(golang.GetLanguage())

// 	sourceCode := []byte(`package main

// import (
// 	"fmt"
// )

// func main() {
// 	fmt.Println("Hello, world!")
// }
// 	`)
// 	tree, err := parser.ParseCtx(context.Background(), nil, sourceCode)
// 	if err != nil {
// 		panic(err)
// 	}

// 	n := tree.RootNode()

// 	traverseTree(n)
// }

// // traverse the tree DFS
// func traverseTree(n *sitter.Node) {
// 	// create cursor

// }

//
// var sourceCode = `
// package main

// import "fmt"

// func add(a, b int) int {
// 	return a + b
// }

// func main() {
// 	sum := add(3, 4)
// 	fmt.Println(sum)
// }
// `

// func main() {
// 	parser := sitter.NewParser()
// 	parser.SetLanguage(golang.GetLanguage())
// 	tree, err := parser.ParseCtx(context.TODO(), nil, []byte(sourceCode))
// 	if err != nil {
// 		fmt.Printf("Error parsing source code: %v", err)
// 		os.Exit(1)
// 	}

// 	// Search for the first function definition
// 	functionNode := findFunctionNode(tree.RootNode())

// 	// Extract symbols from the function
// 	symbols := extractSymbols(functionNode)

// 	fmt.Println("Symbols in the function:")
// 	for _, symbol := range symbols {
// 		fmt.Println(symbol)
// 	}
// }

// func findFunctionNode(node *sitter.Node) *sitter.Node {
// 	if node.Type() == "function_declaration" {
// 		fmt.Println("found function declaration, for node" + node.)
// 		return node
// 	}

// 	for i := 0; i < int(node.ChildCount()); i++ {
// 		childNode := node.Child(i)
// 		result := findFunctionNode(childNode)
// 		if result != nil {
// 			return result
// 		}
// 	}

// 	return nil
// }

// func extractSymbols(node *sitter.Node) []string {
// 	symbols := []string{}

// 	if node.Type() == "identifier" {
// 		symbols = append(symbols, string(node.Content([]byte(sourceCode))))
// 	}

// 	for i := 0; i < int(node.ChildCount()); i++ {
// 		childNode := node.Child(i)
// 		symbols = append(symbols, extractSymbols(childNode)...)
// 	}

// 	return symbols
// }
