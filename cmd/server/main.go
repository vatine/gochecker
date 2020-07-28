package main

import (
	"flag"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/vatine/gochecker/pkg/handlers"
	"github.com/vatine/gochecker/pkg/pkgdata"
)

// Make sure that the package data is saved every so often, in case
// there's an unexepcted temination. Also allows for running various
// tabulation and checks on the data periodically.
func periodicSave(d time.Duration) {
	t := time.NewTicker(d)
	for _ = range t.C {
		pkgdata.Save()
	}
}

func main() {
	var dataDir string
	var image string
	var envFile string
	var endpoint string
	var saveInterval time.Duration
	var verbose bool

	flag.StringVar(&dataDir, "datadir", "/tmp/go_data", "Data directory for long-term storage.")
	flag.StringVar(&image, "image", "gobuilder:manual", "Name of the image to use for go builds")
	flag.StringVar(&envFile, "env-file", "/tmp/go_data/env", "Name of the file to use for the environment file for the build image.")
	flag.StringVar(&endpoint, "endpoint", "http://192.168.1.2:8080/api/report", "Endpoint for reporting build status to.")
	flag.DurationVar(&saveInterval, "interval", time.Hour, "Time between saves")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging")

	flag.Parse()

	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	pkgdata.SetStoragePath(dataDir)
	go periodicSave(saveInterval)
	pkgdata.LoadLatest()

	handlers.VC.Image = image
	handlers.VC.EnvFile = envFile
	handlers.VC.Endpoint = endpoint

	http.HandleFunc("/api/report", handlers.HandleStatusCallback)
	http.HandleFunc("/api/validate", handlers.HandleValidation)
	http.HandleFunc("/api/save", handlers.SaveHandler)

	http.ListenAndServe(":8080", nil)
}
