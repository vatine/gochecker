// Package for the "package data" data structures, so we can
// eventually generate statistics from this.

package pkgdata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	// "sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Various bits of data about a package/module.
type PackageStats struct {
	DownloadSucceeded bool     `json:"downloadSucceeded"`
	BuildableTargets  int      `json:"buildableTargets"`
	AllBuildsPass     bool     `json:"allBuildsPass"`
	TestableTargets   int      `json:"testableTargets"`
	AllTestsPassed    bool     `json:"allTestsPass"`
	VetPassed         []string `json:"passedVets,omitempty"`
	FailedBuilds      []string `json:"failedBuilds,omitempty"`
	FailedTests       []string `json:"failedTests,omitempty"`
	FailedVets        []string `json:"failedVets,omitempty"`
	FailedFmt         []string `json:"failedFmt,omitempty"`
}

// A datatype suitable for iterating on the collected data
type Package struct {
	Name  string
	Stats PackageStats
}

var storagePath string
var dataLock sync.Mutex
var packages map[string]*PackageStats
var clean bool
var cleaned []string

func init() {
	packages = make(map[string]*PackageStats)
	clean = true
}

// Turn a module, version pair into a package name for storage
func BuildPackageName(module, version string) string {
	return fmt.Sprintf("%s@%s", module, version)
}

// Set the directory where state files are kept.
func SetStoragePath(newPath string) error {
	info, err := os.Stat(newPath)
	if err != nil {
		return err
	}

	if !info.Mode().IsDir() {
		return fmt.Errorf("Not a directory, %s", newPath)
	}

	storagePath = newPath
	clean = true

	return nil
}

// Save package state to disk if there's been any changes since last
// save. Mark the data as "clean".
func Save() error {
	dataLock.Lock()
	defer dataLock.Unlock()

	if clean {
		return nil
	}

	filename := fmt.Sprintf("pkgdata-%s", time.Now().Format(time.RFC3339))
	target := filepath.Join(storagePath, filename)

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	b, err := json.Marshal(packages)
	if err != nil {
		return err
	}

	_, err = out.Write(b)

	if err != nil {
		return err
	}
	clean = true

	return nil
}

// Load package state from disk.
func Load(name string) error {

	var intermediate map[string]PackageStats

	source, err := os.Open(name)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"filename": name,
			"error":    err,
		}).Error("Opening file")
		return err
	}
	defer source.Close()

	b, err := ioutil.ReadAll(source)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &intermediate)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("Failed to unmarshal save file.")
		return err
	}

	for key, val := range intermediate {
		EnsurePackage(key)
		SetPackageData(key, val)
	}

	dataLock.Lock()
	defer dataLock.Unlock()
	clean = true

	logrus.WithFields(logrus.Fields{"name": name}).Info("Loading complete.")

	return nil
}

// Load the latest file from disk
func LoadLatest() error {
	pattern := filepath.Join(storagePath, "pkgdata-*")
	names, err := filepath.Glob(pattern)
	logrus.WithFields(logrus.Fields{
		"pattern": pattern,
		"names":   names,
	}).Debug("Loading latest.")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"pattern": pattern,
			"error":   err,
		}).Error("Globbing for latest.")
		return err
	}

	if len(names) == 0 {
		logrus.Info("No save files.")
		return nil
	}

	name := names[len(names)-1]
	logrus.WithFields(logrus.Fields{
		"name": name,
	}).Debug("About to load data.")
	err = Load(name)

	return err
}

// Check if we have seen any data for the named package.
func PackageSeen(name string) bool {
	dataLock.Lock()
	defer dataLock.Unlock()

	_, ok := packages[name]
	return ok
}

// Return a copy of the package data for a given package
func GetPackageData(name string) (PackageStats, bool) {
	dataLock.Lock()
	defer dataLock.Unlock()

	rv, ok := packages[name]

	if ok {
		return *rv, true
	}
	return PackageStats{}, false
}

// If we don't have any data about a given package, initialize an
// empty PackageStats struct and store that. Return false if the
// package didn't exist, otherwise return true.
func EnsurePackage(name string) bool {
	dataLock.Lock()
	defer dataLock.Unlock()

	_, ok := packages[name]
	if !ok {
		packages[name] = new(PackageStats)
		clean = false
		return false
	}

	return true
}

// Set the package stats for a given package. This will set the state
// to "not clean", even if we end up setting the exact same data that
// we already had.
func SetPackageData(name string, data PackageStats) {
	dataLock.Lock()
	defer dataLock.Unlock()

	clean = false
	blob, ok := packages[name]
	if !ok {
		packages[name] = new(PackageStats)
	}

	blob.DownloadSucceeded = data.DownloadSucceeded
	blob.BuildableTargets = data.BuildableTargets
	blob.AllBuildsPass = data.AllBuildsPass
	blob.TestableTargets = data.TestableTargets
	blob.AllTestsPassed = data.AllTestsPassed
	blob.VetPassed = data.VetPassed
	blob.FailedBuilds = data.FailedBuilds
	blob.FailedTests = data.FailedTests
	blob.FailedVets = data.FailedVets
}

// Returns a channel on which all packages with statistics will be
// passed.  This function is NOT concurrency-safe, as it does not lock
// the data.  But for "off-line" use (gathering and emitting
// statistics) this is not a concern.
func AllPackages() chan Package {
	rv := make(chan Package)

	go func() {
		for pkg, stats := range packages {
			rv <- Package{pkg, *stats}
		}
		close(rv)
	}()

	return rv
}

// Drop all "download failed" packages from the statistics, to allow
// for a clean(er) "stop everything, restart" experience.
// Returns the number of packages deleted.
func PurgeDownloadFailed() int {
	dataLock.Lock()
	defer dataLock.Unlock()

	clean = false

	var toDelete []string

	for key, val := range packages {
		if !val.DownloadSucceeded {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(packages, key)
	}

	return len(toDelete)
}

// Delete a specific package from the statistics
func PurgePackage(name string) {
	dataLock.Lock()
	defer dataLock.Unlock()

	clean = false

	delete(packages, name)
}
