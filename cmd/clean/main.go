// It seems as if newer versions of Athens will activate the
// validation webhook, as it steps back trying to find the desired
// path. This causes at least some "download failed" in the stats that
// obscure what's actually happening.
package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/vatine/gochecker/pkg/deciders"
	"github.com/vatine/gochecker/pkg/pkgdata"
)

func clean(pkg pkgdata.Package) bool {
	if pkg.Stats.DownloadSucceeded {
		return false
	}

	if deciders.DomainOnly(pkg) {
		log.WithFields(log.Fields{
			"pkg": pkg.Name,
		}).Info("domain only")
		return true
	}

	if deciders.Banned(pkg) {
		log.WithFields(log.Fields{
			"pkg": pkg.Name,
		}).Info("manual shortlist")
		return true
	}

	if deciders.IncommensurateName(pkg) {
		log.WithFields(log.Fields{
			"pkg": pkg.Name,
		}).Info("version weirdness")
		return true
	}

	return false
}

func main() {
	pkgdata.SetStoragePath("/tmp/go_data")
	pkgdata.LoadLatest()

	var toDel []string

	for pkg := range pkgdata.AllPackages() {
		if clean(pkg) {
			toDel = append(toDel, pkg.Name)
		}
	}

	for _, name := range toDel {
		pkgdata.PurgePackage(name)
	}

	log.WithFields(log.Fields{
		"zapped": len(toDel),
	}).Info("# pkgs deleted")

	err := pkgdata.Save()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("failed to safe DB")
	}
}
