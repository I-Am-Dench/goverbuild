package manifest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/I-Am-Dench/goverbuild/archive/manifest"
)

func testRead(name string, expected *manifest.Manifest) func(*testing.T) {
	return func(t *testing.T) {
		file, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		actual, err := manifest.Read(file)
		if err != nil {
			t.Fatal(err)
		}

		if actual.Version != expected.Version {
			t.Errorf("expected name %s but got %s", expected.Name, actual.Name)
		}

		if len(expected.Entries) != len(actual.Entries) {
			t.Fatalf("expected %d files but got %d", len(expected.Entries), len(actual.Entries))
		}

		for i, actual := range actual.Entries {
			expected := expected.Entries[i]

			if expected.Path != actual.Path {
				t.Errorf("expected %s but got %s", expected.Path, actual.Path)
			}
		}
	}
}

func newManifest(version int, name string, entries []string) *manifest.Manifest {
	m := &manifest.Manifest{Version: version, Name: name}

	for _, entry := range entries {
		m.Entries = append(m.Entries, &manifest.Entry{
			Path: entry,
		})
	}

	return m
}

func TestManifest(t *testing.T) {

	t.Run("index.txt", testRead("index.txt", newManifest(82, "0", []string{
		"frontend.txt",
		"primary.pki",
		"trunk.txt",
	})))

	t.Run("trunk.txt", testRead("trunk.txt", newManifest(82, "1.LU.10.64--CC123", []string{
		"client/Awesomium.dll",
		"client/AwesomiumProcess.exe",
		"client/COP.dll",
		"client/LEGOUniverse.exe",
		"client/LEGOUniverse_Mac.exe",
		"client/Lwoclient.state",
		"client/binkw32.dll",
		"client/boot.cfg",
		"client/d3dx9_34.dll",
		"client/fmod_event.dll",
		"client/fmodex.dll",
		"client/icudt42.dll",
	})))
}
