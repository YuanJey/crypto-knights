package report

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed macro-risk-report.schema.json
var schemaDocument []byte

var compiledSchema = mustCompileSchema()

func validateDocument(document []byte) error {
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return fmt.Errorf("decode report for schema validation: %w", err)
	}
	if err := compiledSchema.Validate(value); err != nil {
		return fmt.Errorf("report does not conform to macro-risk.v1: %w", err)
	}
	return nil
}

func mustCompileSchema() *jsonschema.Schema {
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaDocument))
	if err != nil {
		panic(fmt.Errorf("decode embedded macro report schema: %w", err))
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if resourceError := compiler.AddResource("macro-risk-report.schema.json", schemaValue); resourceError != nil {
		panic(fmt.Errorf("register embedded macro report schema: %w", resourceError))
	}
	schema, compileError := compiler.Compile("macro-risk-report.schema.json")
	if compileError != nil {
		panic(fmt.Errorf("compile embedded macro report schema: %w", compileError))
	}
	return schema
}
