package util

import (
	"testing"

	"github.com/spf13/afero"
)

func TestUniqueFilename_TimestampUnixNano_One(t *testing.T) {
	var TestFs = afero.NewMemMapFs()
	TestFs.Create("1692197030830418000.json")

	got := UniqueFilename(TestFs, "1692197030830418000.json")
	want := "1692197030830418000_1.json"

	if got != want {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func TestUniqueFilename_TimestampUnixNano_Two(t *testing.T) {
	var TestFs = afero.NewMemMapFs()
	TestFs.Create("1692197487593485000.json")
	TestFs.Create("1692197487593485000_1.json")

	got := UniqueFilename(TestFs, "1692197487593485000.json")
	want := "1692197487593485000_2.json"

	if got != want {
		t.Errorf("got %q, wanted %q", got, want)
	}
}

func TestUniqueFilename_Empty(t *testing.T) {
	var TestFs = afero.NewMemMapFs()
	TestFs.Create("")

	got := UniqueFilename(TestFs, "")
	want := ""

	if got != want {
		t.Errorf("got %q, wanted %q", got, want)
	}
}
