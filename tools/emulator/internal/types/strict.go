package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

// StrictYAML is yaml.Unmarshal, but an unknown key or a second document is an error:
// hand-written files would otherwise lose a misspelled key without a word.
func StrictYAML(data []byte, out interface{}) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && err != io.EOF {
		return err
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("only one YAML document is read; remove the extra '---' document")
	}
	return nil
}

// StrictJSON is json.Unmarshal, but an unknown field or anything after the value is an error.
func StrictJSON(data []byte, out interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("unexpected data after the JSON value")
	}
	return nil
}
