#!/bin/bash

set -e

export GOPATH=$(pwd)/gopath
export PATH=$PATH:$GOPATH/bin

cd gopath/src/github.com/cloud-gov/purge-sandboxes

cd cmd/purge

go run .
