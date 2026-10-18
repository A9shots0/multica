package agentqualification

import (
	"os"
	"strings"
	"testing"
)

func TestRealisticFixtureManifestPinsCurrentFixtureTrees(t *testing.T) {
	manifest, err := CurrentRealisticFixtureManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range manifest.Fixtures {
		root := "testdata/realistic/" + strings.ToLower(fixture.CaseID)
		actual, err := RealisticFixtureTreeSHA256(os.DirFS("."), root)
		if err != nil {
			t.Fatalf("hash %s: %v", fixture.CaseID, err)
		}
		if actual != fixture.TreeSHA256 {
			t.Errorf("fixture %s drifted: got %s want %s", fixture.CaseID, actual, fixture.TreeSHA256)
		}
	}
}

func TestRealisticFixtureManifestRejectsPathEscape(t *testing.T) {
	manifest, err := CurrentRealisticFixtureManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Fixtures = append([]RealisticFixture(nil), manifest.Fixtures...)
	manifest.Fixtures[0].EditablePaths = []string{"../owner-file"}
	if err := ValidateRealisticFixtureManifest(manifest); err == nil {
		t.Fatal("fixture manifest accepted path escape")
	}
}
