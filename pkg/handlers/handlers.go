package handlers
// Handlers for web endpoints

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/vatine/gochecker/pkg/pkgdata"
	"github.com/vatine/gochecker/pkg/validation"
)

type PackagePayload struct {
	Package string `json:"package"`
	Data    pkgdata.PackageStats `json:"data"`
}

type ValidationRequest struct {
	Module  string
	Version string
}

var VC validation.ValidationConfiguration

// Update status for a module at a specific version.
func HandleStatusCallback(w http.ResponseWriter, r *http.Request) {
	var payload PackagePayload
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(b, &payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}

	logrus.WithFields(logrus.Fields{"package": payload.Package}).Info("Status update")
	
	pkgdata.SetPackageData(payload.Package, payload.Data)
	logrus.WithFields(logrus.Fields{
		"package": payload.Package,
		"pkgdata": payload.Data,
	}).Debug("Payload details")

	return
}

// Handle an incoming validation request from Athens
func HandleValidation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Unexpected method, %s.", r.Method)
		return
	}

	if c := r.Header.Get("Content-Type"); c != "application/json" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintf(w, "Unexpected Content-Type, %s", c)
		return
	}

	// We're all good...
	var vr ValidationRequest

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(b, &vr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, err)
		return
	}

	pkg := pkgdata.BuildPackageName(vr.Module, vr.Version)
	if !pkgdata.EnsurePackage(pkg) {
		err := VC.Start(vr.Module, vr.Version)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// Make sure we can trigger a save from the outside.
func SaveHandler(w http.ResponseWriter, r *http.Request) {
	err := pkgdata.Save()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Eror saving, %v", err)
	}
	fmt.Fprintln(w, "Save complete")
}
