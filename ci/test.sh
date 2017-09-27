#!/bin/bash

set -e

export GOPATH=$(pwd)/gopath
export PATH=$PATH:$GOPATH/bin

cd gopath/src/github.com/18F/cg-sandbox

go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega

ginkgo -r
