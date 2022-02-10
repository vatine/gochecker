package deciders

import (
	"strings"

	"github.com/vatine/gochecker/pkg/pkgdata"
)

var shortened = []string{
	"honnef.co/go",
	"cloud.google.com/go",
	"dmitri.shuralyov.com/gpu",
	"golang.org/x",
}

func modName(pkg pkgdata.Package) string {
	atPos := strings.Index(pkg.Name, "@")
	if atPos == -1 {
		atPos = len(pkg.Name)
	}

	return pkg.Name[:atPos]
}

// Return true if the version is not v0 or v1, the name does not have
// a corresponding /vN, and there's no "+incompatible" on the end.
func IncommensurateName(pkg pkgdata.Package) bool {
	split := strings.Split(pkg.Name, "@")
	name, version := split[0], split[1]

	switch {
	case !strings.HasPrefix(version, "v"):
		return false
	case strings.HasPrefix(version, "v0"):
		return false
	case strings.HasPrefix(version, "v1"):
		return false
	}

	// We now have a v2 or higher
	if strings.HasSuffix(version, "ible") {
		return false
	}

	split = strings.Split(version, ".")
	vn := split[0]
	components := strings.Split(name, "/")
	if components[len(components)-1] != vn {
		return true
	}

	return false
}

// Return true if the name portion of a package is "just" a domain
func DomainOnly(pkg pkgdata.Package) bool {
	return strings.Index(modName(pkg), "/") == -1
}

// Return true if a package provably isn't the name of a valid package.
func Banned(pkg pkgdata.Package) bool {
	name := modName(pkg)

	if strings.HasPrefix(name, "github.com") {
		if len(strings.Split(name, "/")) == 2 {
			return true
		}
	}

	if strings.HasPrefix(name, "bitbucket.org") {
		if len(strings.Split(name, "/")) == 2 {
			return true
		}
	}

	for _, b := range shortened {
		if name == b {
			return true
		}
	}

	return false
}
