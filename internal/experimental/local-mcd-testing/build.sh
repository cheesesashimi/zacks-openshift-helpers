#!/usr/bin/env bash

# This needs a bunch of tags set to build correctly.
go build -tags='exclude_graphdriver_btrfs containers_image_openpgp' .
