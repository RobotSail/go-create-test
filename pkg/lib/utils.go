package lib

import (
	"path"
)

// Returns the name of the test file for the given file.
// For example, if the given file is "foo.go", the returned
// test file name will be "foo_test.go".
func GetTestFileName(filepath string) string {
	testfileName := path.Base(filepath)
	testfileName = testfileName[:len(testfileName)-len(path.Ext(testfileName))] + "_test" + path.Ext(testfileName)
	return testfileName
}
