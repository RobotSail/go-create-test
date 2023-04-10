package types

import (
	sitter "github.com/smacker/go-tree-sitter"
)

type FunctionDeclaration struct {
	Name string
	Body string
}

type FunctionCall struct {
	Name     string
	Location sitter.Point
}

type DefinitionLocation struct {
	Filepath string
	Start    sitter.Point
	End      sitter.Point
}

type FunctionLocation struct {
	FilePath string
	Point    sitter.Point
}

type Range struct {
	Start sitter.Point
	End   sitter.Point
}
