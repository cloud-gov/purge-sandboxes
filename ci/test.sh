#!/bin/bash

set -e

cd cg-sandbox

go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega

ginkgo -r
