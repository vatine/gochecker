# gochecker
Checking build/test of Go packages

## What is this?

This is code to help answer the question "how is the testability and
buildability of Go code in the wild". It is implemented as a
multi-pronged thing, using [Athens](https://gomods/athens) as a Go
proxy,  the server (in cmd/server) as a validator for Athens, and
the code in the python/ subdirectory as a docker image in which builds
happen.

## Data extraction tool

There's also a tool in cmd/tabulate that extracts various numbers from
the data.

## If you want to run it yourself

You will need to:
* Have docker installed
* Make sure you spin up the athens container, configured to point at the validation server and with a port exposed (otherwise the build wrapper can't reach athens).
* Have a Docker environment file suitable to point the build wrapper at Athens
* Start the server with the relevant arguments (it is hard-coded to ruin on port 8080, if your host's IP is not 192.168.1.2, make sure to pass a URL for the report endpoint with `--endpoint`).
* Build the docker image containing the Python build-wrapper (if you don't call the resulting image `gobuilder:manual`, pass whatever you built and tagged it as with `--image`)

With all of that set up, you can trigger one or more manual seed packages by eiter asking the Athens instance to download them, fake up a validation requiest, or start a build using the gobuilder image.
