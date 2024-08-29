#!/bin/bash

set -e

cd purge-sandboxes

go test ./...
