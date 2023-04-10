#/bin/bash
set -e -o pipefail

# This script talks to gopls to go through a function's definition, identify all of the referenced symbols, and resolve their definitions.
# All of these definitions are then printed to stdout.

########################################
# Find the full definition of a function
# Params:
#  string - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  the full definition of the symbol, e.g. pkg/path/to/file.go:123:456
########################################
function get_definition() {
		printf "Getting definition for '%s'\n" "${1}"
		local path="${1}"
		local definition=$(gopls definition -json "${path}")
		echo "${definition}"
}

########################################
# Find the implementation of a function
# Params:
#  string - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  the full definition of the symbol, e.g. pkg/path/to/file.go:123:456
########################################
function get_implementation() {
		printf "Getting implementation for '%s'\n" "${1}"
		local path="${1}"
		local implementation=$(gopls implementation  "${path}")
		echo "${implementation}"
}


########################################
# Find the function definition range
# Params:
#  string - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  string - the range where the function is defined on
########################################
function get_definition_range() {
	local symbolPath="${1}"
	local line_number=$(echo "${symbolPath}" | sed -E 's/.*:([0-9]+):[0-9]+/\1/')
	# printf "extracted line number: '%s'\n" "${line_number}"
	# split the file path based on the ':' which separates the symbol row+colume from the file path
	local file_path=$(echo "${symbolPath}" | sed -E 's/(.*):[0-9]+:[0-9]+/\1/')
	# printf "using filepath: '%s'\n" "${file_path}"
	local foldingRange=$(gopls folding_ranges "${file_path}")
	# printf "folding range: '%s'\n" "${foldingRange}"
	# find the lines in foldingRange which start with the line number.
	# each line will be in the format :[startline]:[startcol]-[endline]:[endcol]
	local relevantLines=$(echo "${foldingRange}" | grep -E "^${line_number}:")

	# from the relevant lines, find the line number which has the greatest end line number
	local maxEndLine=0
	local maxEndLineLine=""
	# for line in "${relevantLines}"; do
	# 	local endLine=$(echo "${line}" | sed -E 's/.*-([0-9]+):[0-9]+/\1/')
	# 	printf "endLine: '%s'\n" "${endLine}"
	# 	if [ "${endLine}" -gt "${maxEndLine}" ]; then
	# 		maxEndLine="${endLine}"
	# 		maxEndLineLine="${line}"
	# 	fi
	# done
	while read -r line; do
		local endLine=$(echo "${line}" | sed -E 's/.*-([0-9]+):[0-9]+/\1/')
		# printf "endLine: '%s'\n" "${endLine}"
		if [ "${endLine}" -gt "${maxEndLine}" ]; then
			maxEndLine="${endLine}"
			maxEndLineLine="${line}"
		fi
	done <<< "${relevantLines}"


	# return the range from the start line to the end line
	local startLine=$(echo "${maxEndLineLine}" | sed -E 's/([0-9]+):[0-9]+-.*/\1/')
	echo "${line_number}-${maxEndLine}"
}

########################################
# Given a symbol path, e.g. pkg/path/to/file.go:123:456, return only the filepath
# Params:
#  (string) - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  (string) - the filepath, e.g. pkg/path/to/file.go
########################################
function get_file_path() {
	local symbolPath="${1}"
	local file_path=$(echo "${symbolPath}" | sed -E 's/(.*):[0-9]+:[0-9]+/\1/')
	echo "${file_path}"
}

########################################
# Given a symbol path, e.g. pkg/path/to/file.go:123:456, return only the line number
# Params:
#  (string) - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  (string) - the line number, e.g. 123
########################################
function get_line_number() {
	local symbolPath="${1}"
	local line_number=$(echo "${symbolPath}" | sed -E 's/.*:([0-9]+):[0-9]+/\1/')
	echo "${line_number}"
}

########################################
# Given a symbol path, e.g. pkg/path/to/file.go:123:456, return only the column number
# Params:
#  (string) - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  (string) - the column number, e.g. 456
########################################
function get_column_number() {
	local symbolPath="${1}"
	local column_number=$(echo "${symbolPath}" | sed -E 's/.*:[0-9]+:([0-9]+)/\1/')
	echo "${column_number}"
}

########################################
# Given the path to a symbol, return the full definition of the symbol
# Params:
#  (string) - path to the referenced symbol, e.g. pkg/path/to/file.go:123:456
# Returns:
#  (string) - the full definition of the symbol, e.g. "func main() {\n\tfmt.Println(\"Hello, world!\")\n}\n
########################################
function print_definition() {
	# extract the line number from the path
	local symbolPath="${1}"
	local line_number=$(echo "${symbolPath}" | sed -E 's/.*:([0-9]+):[0-9]+/\1/')
	local definitionRange=$(get_definition_range "${symbolPath}")
	local startLine=$(echo "${definitionRange}" | sed -E 's/([0-9]+)-.*/\1/')
	local endLine=$(echo "${definitionRange}" | sed -E 's/[0-9]+-([0-9]+)/\1/')
	# print out the filepath at the given line number
	local filePath=$(get_file_path "${symbolPath}")
	printf "definitionRange: '%s'\n" "${definitionRange}"
	echo $(sed -n "${startLine},${endLine}p" "${filePath}")
}

# get_definition "${1}"
# get_implementation "${1}"
# print_definition "${1}"
print_definition "${1}"
