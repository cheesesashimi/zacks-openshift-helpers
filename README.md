# Zack's OpenShift Helpers

## Background

This repository contains Golang programs which are most likely to be of use to
fellow OpenShift developers, especially members of the
[machine-config-operator](https://github.com/openshift/machine-config-operator)
team. The helpers found here may be of use to you. They may not. They may
completely break entirely.

## Installation

Pre-built binaries for Linux and Mac on both amd64 and aarch64 architectures
are available for download via the [GitHub
Releases](https://github.com/cheesesashimi/zacks-openshift-helpers/releases)
page. All one has to do is download them and place them somewhere in your
`PATH`.

These binaries are built with [goreleaser](https://goreleaser.com/) running as
a GitHub Action. If you're using a Mac, you'll need to [jump through a few
hoops](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac)
to make these work for right now.

In the future, I plan to make each individual binary available for download in
addition to the current archive scheme.

## Further Notes

There is an `experimental` directory which contains purely experimental code. Use at your own risk. 
