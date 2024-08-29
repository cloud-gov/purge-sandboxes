#!/bin/bash

set -e

export GOPATH=$(pwd)/gopath
export PATH=$PATH:$GOPATH/bin

cd gopath/src/github.com/18F/purge-sandboxes

cd cmd/purge

go run .
