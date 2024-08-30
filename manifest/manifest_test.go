package manifest_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/I-Am-Dench/goverbuild/manifest"
)

var test1 = `[version]
82,9778d5d219c5080b9a6a17bef029331c,0
[files]
frontend.txt,49437,a1f0a118da90ec530a63f4d459cb5f15,23526,d570bd4a1d0643aec3d3b20c66c22c4b,45b28f5bebcae9cec7fdd5c84b0777cd
primary.pki,1271893,1bc91d989c3ed3388be4ee11e1921287,504203,e82af974020ea28d7f95f44b51190c07,e647a70ccd52844e205d1322e5d5f410
trunk.txt,10910460,2ab7c538af7ce4e49e0d1921c41098a8,4462609,43ea7227a9f45f00baa05907f650b43b,48f3970d8da730788a97128553b3e8b6`

var test2 = `[version]
82,9778d5d219c5080b9a6a17bef029331c,1.LU.10.64--CC123
[files]
client/Awesomium.dll,21562880,e00896c0ecc03a375dcd54b9b1034b54,10071054,636e8ad30de6e9750fbaee9f7ed6c182,77274354f97e7264c4e6df9c2a941ea2
client/AwesomiumProcess.exe,451176,9c5b7ab68910e0be65419c617b8394fc,195480,847f888f4317191feee82759f32217a2,919a34dab7b25a6e8e9d09473c396709
client/COP.dll,3862528,da50f2b835941cd72b8596fb2716ef44,1596345,70e8c008e01dc397a2d35abf8f36ac03,9e0f49c22e8448cbbcbf21452f97012f
client/LEGOUniverse.exe,23029352,29d6870c6e9229cafd58d0e613d10f89,8481394,d4b613578bd8f507c86059855988eb1a,e43a06b12c744b418fffc7170a41d931
client/LEGOUniverse_Mac.exe,21650024,9c947825f906288074a02111b446a1ef,7802630,35fd88305efe0ebf5fb6f9c8ea3e2b88,36ca7a29d1be735d1733e7b68c8ba948
client/Lwoclient.state,2260,8b9b37ea73e2d242463f697840085b35,669,c61c601c5152f689c0a050c2fec5b09f,75de4434ad6b84c08ba812bbe841718c
client/binkw32.dll,174080,80d353fadd34bb551b912289eae596af,97330,d832c5cd4d7fcea0e63e8ff6d2c8e962,8a37af42851d549028c8785d2713c0f7
client/boot.cfg,113,5d6809dd4a1f4d8b61c7e4861edf98d2,99,b019f9f8cad5eb9848c37f7c19d81b41,4ebff53ee7337034d5e737c2ba937eda
client/d3dx9_34.dll,3497832,1ca939918ed1b930059b3a882de6f648,1603714,4851ffdb3ac951556eaf595999446ad0,8b89cf4fed0e13f4be2fcc6f14b15064
client/fmod_event.dll,307200,203956a75fe0d8ac6906793bdfe0d211,137264,83eafa3aa5458c52bb3a4fb9f848ed08,cd0095c224e32daef3a152ca8173e4be
client/fmodex.dll,843776,7d040207c78542104a8790ab695bc9c0,408777,b4f4dc4865302759d9ceed68679bb115,cc476e28a981ff764a8adce51efff461
client/icudt42.dll,10941440,0c5bd1f7a69a176d6029a8c598a13261,4696736,53fd6c192550d5b0c3776c335d01f724,be71e551060d1503403d4a55443dd3cb`

func testRead(expected *manifest.Manifest, expectedFiles []string, r io.Reader) error {
	actual, err := manifest.Read(r)
	if err != nil {
		return err
	}

	if actual.Version != expected.Version {
		return fmt.Errorf("manifest: expected version %d, but got %d", expected.Version, actual.Version)
	}

	if actual.Name != expected.Name {
		return fmt.Errorf("manifest: expected name %s, but got %s", expected.Name, actual.Name)
	}

	if len(expectedFiles) != len(actual.Files) {
		return fmt.Errorf("manifest: expected %d files, but got %d", len(expectedFiles), len(actual.Files))
	}

	errs := []error{}
	for i, file := range actual.Files {
		if file.Name() != expectedFiles[i] {
			errs = append(errs, fmt.Errorf("manifest: expected %s, but got %s", expectedFiles[i], file.Name()))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func TestManifest(t *testing.T) {
	if err := testRead(
		&manifest.Manifest{Version: 82, Name: "0"},
		[]string{
			"frontend.txt",
			"primary.pki",
			"trunk.txt",
		},
		bytes.NewBuffer([]byte(test1)),
	); err != nil {
		t.Error(err)
	}

	if err := testRead(
		&manifest.Manifest{Version: 82, Name: "1.LU.10.64--CC123"},
		[]string{
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
		},
		bytes.NewBuffer([]byte(test2)),
	); err != nil {
		t.Error(err)
	}
}
