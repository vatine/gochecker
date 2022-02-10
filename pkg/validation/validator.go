package validation

// A package to ensure that we can spin up the external validator

import (
	"os/exec"

	"github.com/sirupsen/logrus"
)

type ValidationConfiguration struct {
	Image    string
	EnvFile  string
	Endpoint string
}

type execBlob struct {
	args []string
}

var execChan chan execBlob

func init() {
	execChan = startRunLoop()
}

// Starts multiple loops consuming one execution item at a time. Returns the
// chan execBlob that things to be run will be sent over.
func startRunLoop() chan execBlob {
	c := make(chan execBlob)

	go runLoop(c)
	go runLoop(c)
	go runLoop(c)

	return c
}

// Runs a loop, consuming one item at a time from c, finishing (if at
// all) when c is closed.
func runLoop(c chan execBlob) {
	for blob := range c {
		logrus.WithFields(logrus.Fields{
			"args": blob.args,
		}).Info("Spawning external checker.")
		cmd := exec.Command(blob.args[0], blob.args[1:]...)

		err := cmd.Start()
		if err != nil {
			logrus.WithFields(logrus.Fields{}).Error("Failed to spawn external command.")
		} else {
			cmd.Wait()
			logrus.WithFields(logrus.Fields{
				"args": blob.args,
			}).Info("Check complete")
		}
	}
}

// Start an externa validation run, this essentialy boils down to
// doing a docker run of the image we care about, with the environment
// file we care about.
// Return immediately, spawn a goroutine to wait for the child process
func (c ValidationConfiguration) Start(module, version string) error {
	go func() {
		execChan <- execBlob{
			[]string{
				"docker", "run", "--rm", "--env-file",
				c.EnvFile, c.Image, module, version, c.Endpoint,
			},
		}
	}()
	return nil
}
