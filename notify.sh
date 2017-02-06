#!/bin/bash

set -e

export GOPATH=$(pwd)/gopath
export PATH=$PATH:$GOPATH/bin

go get github.com/Masterminds/glide

cd gopath/src/github.com/18F/cg-sandbox

glide install
go run notify.go
