package parse

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"

	"github.com/robotsail/go-create-test/pkg/types"
)

const queryPattern = `
(
  (comment) @function.comment
  (function_declaration
    name: (identifier) @function.name
    body: (block) @function.body)
)
`

// findFunction attempts to find a function with the target name in the given source tree.
// The root declaration node is returned.
func findFunction(functionName string, t *sitter.Node, source []byte) *sitter.Node {
	if t == nil {
		return nil
	}
	if t.Type() == "function_declaration" {
		nameNode := t.ChildByFieldName("name")
		if nameNode == nil {
			return nil
		}
		if nodeName(nameNode, source) == functionName {
			return t
		}
	}
	for i := 0; i < int(t.ChildCount()); i++ {
		childNode := t.Child(i)
		funcNode := findFunction(functionName, childNode, source)
		if funcNode != nil {
			return funcNode
		}
	}
	// move onto next sibling
	return findFunction(functionName, t.NextSibling(), source)
}

// GetFunctionCalls takes a given function name and file to look at, then
// returns the definitions of all of the symbols referred to by that function.
func GetFunctionCalls(filepath string, functionName string, code []byte) ([]string, error) {
	log.Println("parsing code")
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, code)
	if tree == nil {
		if err == nil {
			err = fmt.Errorf("tree is nil")
		}
		return nil, fmt.Errorf("could not parse code: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("could not parse code: %w", err)
	}
	defer tree.Close()

	targetFunction := findFunction(functionName, tree.RootNode(), code)
	if targetFunction == nil {
		return nil, fmt.Errorf("could not find function %q", functionName)
	}

	functionCalls := findFunctionCalls(targetFunction, code)
	for _, call := range functionCalls {
		log.Printf("Found function call '%s' at '%d:%d'\n", nodeName(call, code), call.StartPoint().Row, call.StartPoint().Column)
	}

	functionDefs, err := findDefinitions(filepath, functionCalls, code)
	if err != nil {
		return nil, fmt.Errorf("error finding definitions: %v", err)
	}
	defs, err := readFunctionDefinitions(functionDefs)
	if err != nil {
		return nil, fmt.Errorf("error reading function definitions: %v", err)
	}
	return defs, nil
}

// nodeName returns the name of the given node.
func nodeName(t *sitter.Node, source []byte) string {
	return string(t.Content([]byte(source)))
}

// functionDefinitionFromSelector returns the function definition from a selector expression.
// e.g. 'fmt.Println' returns the node containing 'Println'.
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

// getFunctionLocation Returns the location of the function definition.
func getFunctionLocation(t *sitter.Node, source []byte) *sitter.Node {
	log.Printf("checking function, type is %s\n", t.Type())
	if t.Type() == "identifier" {
		log.Printf("Found function '%s' at '%d:%d'\n", nodeName(t, source), t.StartPoint().Row, t.StartPoint().Column)
		return t
	}
	if t.Type() == "selector_expression" {
		funcDef := functionDefinitionFromSelector(t, source)
		log.Printf("function '%s' of '%s' is located at '%d:%d'\n", nodeName(funcDef, source), nodeName(t, source), funcDef.StartPoint().Row, funcDef.StartPoint().Column)
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

// DefinitionStringFromGopls accepts the output from a gopls definition command
// and returns only the file path and line number
// e.g. "'/Users/osilkin/Programming/playground/go-create-test/example/test.go:8:6-14: defined here as func fortnite() string"
// becomes "/Users/osilkin/Programming/playground/go-create-test/example/test.go:8:6-14"
func DefinitionStringFromGopls(definition string) (string, sitter.Point, error) {
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

func findDefinitions(filename string, calls []*sitter.Node, code []byte) ([]types.DefinitionLocation, error) {
	definitions := []types.DefinitionLocation{}
	for _, call := range calls {
		params := fmt.Sprintf("%s:%d:%d", filename, call.StartPoint().Row+1, call.StartPoint().Column+1)
		command := exec.Command("gopls", "definition", params)
		out, err := command.Output()
		if err != nil {
			stderr, ok := err.(*exec.ExitError)
			if ok {
				log.Printf("error running gopls definition, params used: %s\n", params)
				return nil, fmt.Errorf("error running gopls definition: '%s', error: %w", string(stderr.Stderr), err)
			}
			return nil, fmt.Errorf("error running gopls definition: %w", err)
		}
		filepath, location, err := DefinitionStringFromGopls(string(out))
		if err != nil {
			return nil, fmt.Errorf("error parsing gopls definition: %w", err)
		}
		definitionRange, err := getDefinitionRange(filepath, location)
		if err != nil {
			return nil, fmt.Errorf("error getting definition range: %w", err)
		}
		log.Printf("definition range for '%s' is '%+v'\n", nodeName(call, code), definitionRange)
		definitions = append(definitions, types.DefinitionLocation{
			Filepath: filepath,
			Start:    definitionRange.Start,
			End:      definitionRange.End,
		})
	}
	return definitions, nil
}

// getSplitRange takes a range string like 'row:col-row:col' and returns it in a serialized format.
func getSplitRange(rangeString string) (types.Range, error) {
	// validate that string matches row:col-row:col
	if !strings.Contains(rangeString, "-") {
		return types.Range{}, fmt.Errorf("invalid range string: %s", rangeString)
	}
	splitRange := strings.Split(rangeString, "-")
	start := strings.Split(splitRange[0], ":")
	end := strings.Split(splitRange[1], ":")
	startRow, err := strconv.ParseUint(start[0], 10, 32)
	if err != nil {
		return types.Range{}, fmt.Errorf("invalid start row: %s", start[0])
	}
	startCol, err := strconv.ParseUint(start[1], 10, 32)
	if err != nil {
		return types.Range{}, fmt.Errorf("invalid start column: %s", start[1])
	}
	endRow, err := strconv.ParseUint(end[0], 10, 32)
	if err != nil {
		return types.Range{}, fmt.Errorf("invalid end row: %s", end[0])
	}
	endCol, err := strconv.ParseUint(end[1], 10, 32)
	if err != nil {
		return types.Range{}, fmt.Errorf("invalid end column: %s", end[1])
	}
	return types.Range{
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

// getDefinitionRange finds the function definition range.
//
//	string - the range where the function is defined on
func getDefinitionRange(filepath string, location sitter.Point) (types.Range, error) {
	foldingRangeCmd := exec.Command("gopls", "folding_ranges", filepath)
	foldingRangeBytes, err := foldingRangeCmd.Output()
	if err != nil {
		return types.Range{}, fmt.Errorf("error running gopls folding_ranges: %w", err)
	}
	foldingRange := string(foldingRangeBytes)
	foldingRange = strings.TrimSpace(foldingRange)
	foldingRanges := strings.Split(foldingRange, "\n")

	// convert to ranges
	ranges := []types.Range{}
	for _, rangeLine := range foldingRanges {
		symbolRange, err := getSplitRange(rangeLine)
		if err != nil {
			log.Printf("could not parse range: %v\n", err)
			continue
		}
		ranges = append(ranges, symbolRange)
	}

	// collect only the matching ranges
	biggestRange := types.Range{}
	for i := 0; i < len(ranges); i++ {
		if ranges[i].Start.Row == location.Row && ranges[i].End.Row > biggestRange.End.Row {
			biggestRange = ranges[i]
		}
	}
	return biggestRange, nil
}

func getFunctionComments(fileLines []string, start int) (comments []string) {
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

func readFunctionDefinitions(defs []types.DefinitionLocation) ([]string, error) {
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
		commentLines := getFunctionComments(fileLines, startIndex-2)
		functionDef := append(commentLines, functionBody...)
		function := strings.Join(functionDef, "\n")
		contents = append(contents, function)
	}
	return contents, nil
}

// GetFunctionDefinition Returns the definition of a given function within a given file.
func GetFunctionDefinition(functionName string, code []byte) (string, error) {
	// create a tree-sitter parser
	log.Println("getting function definition")
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, code)
	if tree == nil {
		if err == nil {
			err = fmt.Errorf("tree is nil")
		}
		return "", fmt.Errorf("could not parse code: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("could not parse code: %w", err)
	}
	defer tree.Close()

	query, err := sitter.NewQuery([]byte(queryPattern), golang.GetLanguage())
	if err != nil {
		log.Fatal(err)
	}

	root := tree.RootNode()
	queryCursor := sitter.NewQueryCursor()
	defer queryCursor.Close()
	queryCursor.Exec(query, root)

	var startByte, endByte uint32
	for {
		match, ok := queryCursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			if capture.Index == 0 {
				startByte = capture.Node.StartByte()
			}
			endByte = capture.Node.EndByte()
		}
	}
	return string(code[startByte:endByte]), nil
}

// GetPackageName Queries the given file for the package name.
func GetPackageName(code []byte) (string, error) {
	// create a tree-sitter parser
	log.Println("getting package name")
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, code)
	defer tree.Close()
	if tree == nil {
		if err == nil {
			err = fmt.Errorf("tree is nil")
		}
		return "", fmt.Errorf("could not parse code: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("could not parse code: %w", err)
	}

	// create a query to extract the golang package name from the code
	const packageQuery = `(package_clause (package_identifier) @name)`
	query, err := sitter.NewQuery([]byte(packageQuery), golang.GetLanguage())
	if err != nil {
		return "", fmt.Errorf("could not create query: %w", err)
	}

	root := tree.RootNode()
	queryCursor := sitter.NewQueryCursor()
	defer queryCursor.Close()
	queryCursor.Exec(query, root)

	var startByte, endByte uint32
	for {
		match, ok := queryCursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			if capture.Index == 0 {
				startByte = capture.Node.StartByte()
			}
			endByte = capture.Node.EndByte()
		}
	}
	return string(code[startByte:endByte]), nil
}
