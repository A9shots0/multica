// continuity-pilot only plans and evaluates experiments. It never calls a model provider.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/multica-ai/multica/server/internal/continuitypilot"
)

func main() {
	mode := flag.String("mode", "", "plan or evaluate")
	input := flag.String("input", "-", "JSON input file, or - for stdin")
	flag.Parse()
	if err := run(*mode, *input, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(mode, input string, output io.Writer) error {
	reader := io.Reader(os.Stdin)
	if input != "-" {
		file, err := os.Open(input)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	switch mode {
	case "plan":
		var config continuitypilot.Config
		if err := decodeStrict(reader, &config); err != nil {
			return err
		}
		plan, err := continuitypilot.Generate(config)
		if err != nil {
			return err
		}
		return encode(output, plan)
	case "evaluate":
		var input continuitypilot.EvaluationInput
		if err := decodeStrict(reader, &input); err != nil {
			return err
		}
		result, err := continuitypilot.Evaluate(input)
		if err != nil {
			return err
		}
		return encode(output, result)
	default:
		return fmt.Errorf("-mode must be plan or evaluate")
	}
}

func decodeStrict(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func encode(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
