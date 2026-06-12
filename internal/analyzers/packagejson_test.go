package analyzers

import "testing"

func TestParsePackageJSON(t *testing.T) {
	info, err := ParsePackageJSON([]byte(`{"name":"demo","scripts":{"build":"vite build"},"dependencies":{"react":"latest"},"devDependencies":{"vite":"latest"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "demo" || info.Scripts["build"] == "" || info.Dependencies["react"] == "" {
		t.Fatalf("unexpected package info: %#v", info)
	}
}
