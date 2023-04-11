# go-create-test

`go-create-test` is a CLI tool that leverages the OpenAI API to automatically generate test code for your Go functions. The tool is built using Cobra and provides a simple interface for generating test code based on the input file and function name.

## Installation

First, ensure you have Go installed on your system. You can download Go from [the official website](https://golang.org/dl/).

Next, use the following command to install the CLI tool:

```bash
go install github.com/RobotSail/go-create-test
```

## Usage

The CLI tool provides only one command:

- `create-test`: Generates a test for a given function within the provided file

Here's a quick example of how to use the create-test command:

```bash
go-create-test create-test -f /path/to/your/file.go -n YourFunctionName
```

### Flags

The command currently accepts the following flags:

`-f`, `--filepath` (string): Path to the file containing the functions to be tested
`-n`, `--function` (string): Name of the function to be tested (only required for the create-test command)


## Contributing

Feel free to open issues or submit pull requests if you'd like to contribute to the project. Contributions are welcome!


## License
This project is licensed under the Apache 2 License - see the [LICENSE](./LICENSE) file for details.




