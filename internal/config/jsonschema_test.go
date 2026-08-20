package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCommittedJSONSchemaIsCurrent(t *testing.T) {
	t.Parallel()
	document, err := JSONSchemaDocument()
	if err != nil {
		t.Fatalf("JSONSchemaDocument() error = %v", err)
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "metabasis.schema.json"))
	if err != nil {
		t.Fatalf("read committed schema: %v", err)
	}
	if !bytes.Equal(document, committed) {
		t.Fatal("metabasis.schema.json is stale; run mise run generate")
	}
}
