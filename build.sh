#!/bin/sh
# Installing mage
go install github.com/magefile/mage
# Load project version
PKG_VERSION=$(cat ./package.json | jq -r ".version")
# Compile it again
echo "Compiling code..."
yarn install --frozen-lockfile && yarn build && mage -v
