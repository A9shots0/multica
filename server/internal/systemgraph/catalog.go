package systemgraph

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

//go:embed default_graph.json
var defaultGraphJSON []byte

var (
	defaultOnce    sync.Once
	defaultCatalog Catalog
	defaultErr     error
)

func Default() (Catalog, error) {
	defaultOnce.Do(func() {
		decoder := json.NewDecoder(bytes.NewReader(defaultGraphJSON))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&defaultCatalog); err != nil {
			defaultErr = fmt.Errorf("decode embedded system graph: %w", err)
			return
		}
		if err := ensureJSONEOF(decoder); err != nil {
			defaultErr = fmt.Errorf("decode embedded system graph: %w", err)
			return
		}
		if err := defaultCatalog.Validate(); err != nil {
			defaultErr = fmt.Errorf("validate embedded system graph: %w", err)
		}
	})
	return defaultCatalog, defaultErr
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
