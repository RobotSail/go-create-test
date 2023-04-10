package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/robotsail/go-create-test/pkg/cmd"
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
	// fileContents, err := LoadTestFile()
	// if err != nil {
	// 	fmt.Println(err)
	// 	os.Exit(1)
	// }
	// err = parseCode(fileContents)
	// if err != nil {
	// 	fmt.Printf("Error parsing source code: %v", err)
	// 	os.Exit(1)
	// }
	rootCmd := cmd.NewGenerateTestCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		os.Exit(0)
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
		fmt.Printf("Found function call '%s' at '%d:%d'\n", nodeName(call, code), call.StartPoint().Row, call.StartPoint().Column)
	}

	defs, err := findDefinitions(functionCalls, code)
	if err != nil {
		return fmt.Errorf("error finding definitions: %v", err)
	}
	functionDefs, err := ReadFunctionDefinitions(defs)
	if err != nil {
		return fmt.Errorf("error reading function definitions: %v", err)
	}
	// print out the function definitions
	for _, def := range functionDefs {
		fmt.Printf("definition:\n%s\n\n", def)
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

func nodeName(t *sitter.Node, source []byte) string {
	return string(t.Content([]byte(source)))
}

func functionDefinitionFromSelector(t *sitter.Node, source []byte) *sitter.Node {
	if t.Type() != "selector_expression" {
		return nil
	}
	for i := 0; i < int(t.ChildCount()); i++ {
		childNode := t.Child(i)
		if childNode.Type() == "field_identifier" {
			return childNode
		}
	}
	return nil
}

func getFunctionLocation(t *sitter.Node, source []byte) *sitter.Node {
	fmt.Printf("checking function, type is %s\n", t.Type())
	if t.Type() == "identifier" {
		fmt.Printf("Found function '%s' at '%d:%d'\n", nodeName(t, source), t.StartPoint().Row, t.StartPoint().Column)
		return t
	}
	if t.Type() == "selector_expression" {
		funcDef := functionDefinitionFromSelector(t, source)
		fmt.Printf("function '%s' of '%s' is located at '%d:%d'\n", nodeName(funcDef, source), nodeName(t, source), funcDef.StartPoint().Row, funcDef.StartPoint().Column)
		return funcDef
	}
	return nil
}

// findFunctionCalls Returns a list of function calls in the source code
func findFunctionCalls(t *sitter.Node, source []byte) []*sitter.Node {
	calls := []*sitter.Node{}
	if t.Type() == "call_expression" {
		child := t.Child(0)
		location := getFunctionLocation(child, source)
		if location == nil {
			return nil
		}
		calls = append(calls, location)
	}
	for i := 0; i < int(t.ChildCount()); i++ {
		childNode := t.Child(i)
		functionCalls := findFunctionCalls(childNode, source)
		if len(functionCalls) > 0 {
			calls = append(calls, functionCalls...)
		}
	}
	return calls
}

type DefinitionLocation struct {
	Filepath string
	Start    sitter.Point
	End      sitter.Point
}

// definitionStringFromGopls accepts the output from a gopls definition command
// and returns only the file path and line number
// e.g. "'/Users/osilkin/Programming/playground/go-create-test/example/test.go:8:6-14: defined here as func fortnite() string"
// becomes "/Users/osilkin/Programming/playground/go-create-test/example/test.go:8:6-14"
func definitionStringFromGopls(definition string) (string, sitter.Point, error) {
	def := strings.Split(definition, " ")
	// remove end ':' from the file path
	symbolPath := def[0]
	symbolPath = strings.TrimRight(symbolPath, ":")
	splitPath := strings.Split(symbolPath, ":")
	if len(splitPath) != 3 {
		return "", sitter.Point{}, fmt.Errorf("invalid definition string: %s", definition)
	}
	filepath := splitPath[0]
	line := splitPath[1]
	lineInt, err := strconv.ParseUint(line, 10, 32)
	if err != nil {
		return "", sitter.Point{}, fmt.Errorf("invalid line number: %s", line)
	}
	columnRange := splitPath[2]
	columnRangeSplit := strings.Split(columnRange, "-")
	columnStart := columnRangeSplit[0]
	columnInt, err := strconv.ParseUint(columnStart, 10, 32)
	if err != nil {
		return "", sitter.Point{}, fmt.Errorf("invalid column number: %s", columnStart)
	}
	startPoint := sitter.Point{
		Row:    uint32(lineInt),
		Column: uint32(columnInt),
	}
	return filepath, startPoint, nil
}

func findDefinitions(calls []*sitter.Node, code []byte) ([]DefinitionLocation, error) {
	definitions := []DefinitionLocation{}
	for _, call := range calls {
		params := fmt.Sprintf("%s:%d:%d", testGoFile, call.StartPoint().Row+1, call.StartPoint().Column+1)
		out, err := exec.Command("gopls", "definition", params).Output()
		if err != nil {
			return nil, fmt.Errorf("error running gopls definition: %v", err)
		}
		filepath, location, err := definitionStringFromGopls(string(out))
		if err != nil {
			return nil, fmt.Errorf("error parsing gopls definition: %v", err)
		}
		definitionRange, err := GetDefinitionRange(filepath, location)
		if err != nil {
			return nil, fmt.Errorf("error getting definition range: %v", err)
		}
		fmt.Printf("definition range for '%s' is '%+v'\n", nodeName(call, code), definitionRange)
		definitions = append(definitions, DefinitionLocation{
			Filepath: filepath,
			Start:    definitionRange.Start,
			End:      definitionRange.End,
		})
	}
	return definitions, nil
}

type FunctionLocation struct {
	FilePath string
	Point    sitter.Point
}

// // Returns a struct containing information encoded in the location string.
// // Location string is just the result of the output of gpls definition, e.g.
// // 'Definition for 'Println' is at '/Users/osilkin/Programming/playground/go-create-test/example/test.go:12:6-21: defined here as func rainbowSixSiege() string'
// func getDefinitionLocation(location string) (FunctionLocation, error) {
// 	// get the filepath from the location
// 	fileInfo := strings.Split(location, " ")[0]
// 	// separate the line and column numbers
// 	// TODO: handle the case when there is a ':' in the path
// 	lineAndColumn := strings.Split(fileInfo, ":")
// 	if len(lineAndColumn) != 3 {
// 		return FunctionLocation{}, fmt.Errorf("invalid location string: %s", location)
// 	}
// 	filepath := lineAndColumn[0]
// 	row := lineAndColumn[1]
// 	columnRange := lineAndColumn[2]
// 	// convert row & column into uint32
// 	rowInt, err := strconv.ParseUint(row, 10, 32)
// 	if err != nil {
// 		return FunctionLocation{}, fmt.Errorf("invalid row number: %s", row)
// 	}
// 	// rowInt32 := uint32(rowInt)
// 	// columnRange, err := strconv.ParseUint(columnRange, 10, 32)

// 	if err != nil {
// 		return FunctionLocation{}, fmt.Errorf("invalid column number: %s", columnRange)
// 	}
// 	return FunctionLocation{
// 		FilePath: filepath,
// 		Point: sitter.Point{
// 			Row:    row,
// 			Column: columnRange,
// 		},
// 	}, nil
// }

type Range struct {
	Start sitter.Point
	End   sitter.Point
}

// getSplitRange takes a range string like 'row:col-row:col' and returns it in a serialized format.
func getSplitRange(rangeString string) (Range, error) {
	// validate that string matches row:col-row:col
	if !strings.Contains(rangeString, "-") {
		return Range{}, fmt.Errorf("invalid range string: %s", rangeString)
	}
	splitRange := strings.Split(rangeString, "-")
	start := strings.Split(splitRange[0], ":")
	end := strings.Split(splitRange[1], ":")
	startRow, err := strconv.ParseUint(start[0], 10, 32)
	if err != nil {
		return Range{}, fmt.Errorf("invalid start row: %s", start[0])
	}
	startCol, err := strconv.ParseUint(start[1], 10, 32)
	if err != nil {
		return Range{}, fmt.Errorf("invalid start column: %s", start[1])
	}
	endRow, err := strconv.ParseUint(end[0], 10, 32)
	if err != nil {
		return Range{}, fmt.Errorf("invalid end row: %s", end[0])
	}
	endCol, err := strconv.ParseUint(end[1], 10, 32)
	if err != nil {
		return Range{}, fmt.Errorf("invalid end column: %s", end[1])
	}
	return Range{
		Start: sitter.Point{
			Row:    uint32(startRow),
			Column: uint32(startCol),
		},
		End: sitter.Point{
			Row:    uint32(endRow),
			Column: uint32(endCol),
		},
	}, nil
}

// GetDefinitionRange finds the function definition range.
//
//	string - the range where the function is defined on
func GetDefinitionRange(filepath string, location sitter.Point) (Range, error) {
	foldingRangeCmd := exec.Command("gopls", "folding_ranges", filepath)
	foldingRangeBytes, err := foldingRangeCmd.Output()
	if err != nil {
		return Range{}, fmt.Errorf("error running gopls folding_ranges: %w", err)
	}
	foldingRange := string(foldingRangeBytes)
	foldingRange = strings.TrimSpace(foldingRange)
	foldingRanges := strings.Split(foldingRange, "\n")

	// convert to ranges
	ranges := []Range{}
	for _, rangeLine := range foldingRanges {
		symbolRange, err := getSplitRange(rangeLine)
		if err != nil {
			fmt.Printf("could not parse range: %v\n", err)
			continue
		}
		ranges = append(ranges, symbolRange)
	}

	// collect only the matching ranges
	biggestRange := Range{}
	for i := 0; i < len(ranges); i++ {
		if ranges[i].Start.Row == location.Row && ranges[i].End.Row > biggestRange.End.Row {
			biggestRange = ranges[i]
		}
	}
	return biggestRange, nil
}

func GetFunctionComments(fileLines []string, start int) (comments []string) {
	for i := start; i >= 0 && i < len(fileLines); i-- {
		if strings.HasPrefix(fileLines[i], "//") {
			comments = append(comments, fileLines[i])
		} else {
			break
		}
	}
	// reverse list so that comments appear in the correct order
	for i, j := 0, len(comments)-1; i < j; i, j = i+1, j-1 {
		comments[i], comments[j] = comments[j], comments[i]
	}
	return
}

func ReadFunctionDefinitions(defs []DefinitionLocation) ([]string, error) {
	contents := []string{}
	for _, def := range defs {
		// read in the given filepath and get the function definition
		file, err := ioutil.ReadFile(def.Filepath)
		if err != nil {
			return nil, fmt.Errorf("could not open file: %w", err)
		}
		// extract the content at the range specified
		fileStr := string(file)
		fileLines := strings.Split(fileStr, "\n")
		if len(fileLines) < int(def.End.Row) {
			return nil, fmt.Errorf("file does not have enough lines: %s", def.Filepath)
		}
		functionBody := fileLines[def.Start.Row-1 : def.End.Row]
		// include the comments
		startIndex := int(def.Start.Row)
		commentLines := GetFunctionComments(fileLines, startIndex-2)
		functionDef := append(commentLines, functionBody...)
		function := strings.Join(functionDef, "\n")
		contents = append(contents, function)
	}
	return contents, nil
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
