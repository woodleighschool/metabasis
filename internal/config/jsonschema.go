package config

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/jsonschema"
)

//go:generate go run ../../cmd/metabasis schema --output ../../metabasis.schema.json

// JSONSchema returns the editor-facing structural schema generated from Config's YAML tags.
func JSONSchema() *jsonschema.Schema {
	durationType := reflect.TypeFor[Duration]()
	reflector := &jsonschema.Reflector{
		FieldNameTag: "yaml",
		Mapper: func(valueType reflect.Type) *jsonschema.Schema {
			if valueType == durationType {
				return &jsonschema.Schema{
					Type:        "string",
					Description: "A Go duration such as 30s, 1m, or 15m.",
					Examples:    []any{"30s", "1m", "15m"},
				}
			}
			return nil
		},
	}
	schema := reflector.Reflect(&Config{})
	schema.ID = jsonschema.ID("https://raw.githubusercontent.com/woodleighschool/metabasis/main/metabasis.schema.json")
	schema.Title = "Metabasis configuration"
	schema.Description = "PostgreSQL, webhook, identity, and temporal group reconciliation policy."
	return schema
}

// JSONSchemaDocument returns the generated schema as stable indented JSON.
func JSONSchemaDocument() ([]byte, error) {
	document, err := json.MarshalIndent(JSONSchema(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(document, '\n'), nil
}
