package deciders

import (
	"testing"

	"github.com/vatine/gochecker/pkg/pkgdata"
)

func fakePackage(name string) pkgdata.Package {
	return pkgdata.Package{Name: name}
}

func TestDomainOnly(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"github.com@foo", true}, {"github.com/foo@foo", false},
	}

	for ix, c := range cases {
		got := DomainOnly(fakePackage(c.name))
		if got != c.want {
			t.Errorf("Case #%d, got %v, want %v", ix, got, c.want)
		}
	}
}

func TestIncommensurateName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"example.com/code/v1@v2.0.0", true},
		{"example.com/code/v2@v2.0.0", false},
		{"example.com/code/v3@v2.0.0", true},
		{"example.com/code@v0.0.0", false},
		{"example.com/code@v1.0.0", false},
		{"example.com/code@v2.0.0", true},
		{"example.com/code@v2.0.0+incompatible", false},
	}
	for ix, c := range cases {
		got := IncommensurateName(fakePackage(c.name))
		if got != c.want {
			t.Errorf("Case #%d, got %v, want %v", ix, got, c.want)
		}
	}
}
