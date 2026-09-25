package types

import "testing"

func TestStrictYAML(t *testing.T) {
	var out struct {
		Name string `yaml:"name"`
	}
	if err := StrictYAML([]byte("name: a\n"), &out); err != nil || out.Name != "a" {
		t.Errorf("valid YAML: %v %q", err, out.Name)
	}
	if err := StrictYAML([]byte(""), &out); err != nil {
		t.Errorf("an empty file is no document, not an error: %v", err)
	}
	for _, bad := range []string{"nmae: a\n", "name: a\n---\nname: b\n"} {
		if err := StrictYAML([]byte(bad), &out); err == nil {
			t.Errorf("%q should be an error", bad)
		}
	}
}

func TestStrictJSON(t *testing.T) {
	var out struct {
		Key string `json:"key"`
	}
	if err := StrictJSON([]byte(`{"key":"a"}`), &out); err != nil || out.Key != "a" {
		t.Errorf("valid JSON: %v %q", err, out.Key)
	}
	for _, bad := range []string{`{"kye":"a"}`, `{"key":"a"} {"key":"b"}`} {
		if err := StrictJSON([]byte(bad), &out); err == nil {
			t.Errorf("%q should be an error", bad)
		}
	}
}
