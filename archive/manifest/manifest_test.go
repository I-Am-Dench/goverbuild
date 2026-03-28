package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
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

		expectedEntries := expected.Entries()
		actualEntries := actual.Entries()

		if len(expectedEntries) != len(actualEntries) {
			t.Fatalf("expected %d files but got %d", len(expectedEntries), len(actualEntries))
		}

		for _, expectedEntry := range expectedEntries {
			actualEntry, ok := actual.GetEntry(expectedEntry.Path)
			if !ok {
				t.Errorf("manifest does not contain %q", expectedEntry.Path)
				continue
			}

			if expectedEntry.Path != actualEntry.Path {
				t.Errorf("expected path %q but got %q", expectedEntry.Path, actualEntry.Path)
			}
		}
	}
}

func testBackslash(name string, expected []string) func(*testing.T) {
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

		for _, expectedEntry := range expected {
			actualEntry, ok := actual.GetEntry(expectedEntry)
			if !ok {
				t.Errorf("manifest does not contain %s", expectedEntry)
				continue
			}

			if toSlash := strings.ReplaceAll(expectedEntry, "\\", "/"); !strings.EqualFold(toSlash, actualEntry.Path) {
				t.Errorf("expected path %q but got %q", toSlash, actualEntry.Path)
			}
		}
	}
}

func newManifest(version int, name string, names []string) *manifest.Manifest {
	m := &manifest.Manifest{Version: version, Name: name}

	entries := []manifest.Entry{}
	for _, n := range names {
		entries = append(entries, manifest.Entry{Path: n})
	}
	m.AddEntries(entries...)

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
		"client/res/brickmodels/newcontent/40335_space_rocket_ride.lxfml",
		"client/res/brickmodels/newcontent/brick_wall_001.lxfml",
		"client/res/brickmodels/newcontent/brick_wall_001_end.lxfml",
		"client/res/brickmodels/newcontent/ff_cottage_workshop.lxfml",
		"client/res/brickmodels/newcontent/ff_horse_cart.lxfml",
		"client/res/brickmodels/newcontent/forest_pillar_w_flag.lxfml",
		"client/res/brickmodels/newcontent/garden_fence_wood_01.lxfml",
		"client/res/brickmodels/newcontent/garden_fence_wood_corner_01.lxfml",
		"client/res/brickmodels/newcontent/garden_picnicbench.lxfml",
		"client/res/brickmodels/newcontent/garden_picnictable_01.lxfml",
	})))

	t.Run("trunk.txt", testBackslash("trunk.txt", []string{
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
		"client/res/brickmodels/newcontent/40335_space_rocket_ride.lxfml",
		"client/res/brickmodels/newcontent/brick_wall_001.lxfml",
		"client/res/brickmodels/newcontent/brick_wall_001_end.lxfml",
		"client/res/brickmodels/newcontent/ff_cottage_workshop.lxfml",
		"client/res/brickmodels/newcontent/ff_horse_cart.lxfml",
		"client/res/brickmodels/newcontent/forest_pillar_w_flag.lxfml",
		"client/res/brickmodels/newcontent/garden_fence_wood_01.lxfml",
		"client/res/brickmodels/newcontent/garden_fence_wood_corner_01.lxfml",
		"client/res/brickmodels/newcontent/garden_picnicbench.lxfml",
		"client/res/brickmodels/newcontent/garden_picnictable_01.lxfml",
	}))
}
