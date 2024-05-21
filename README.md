# Zack's OpenShift Helpers

## Background

This repository contains Golang programs which are most likely to be of use to
fellow OpenShift developers, especially members of the
[machine-config-operator](https://github.com/openshift/machine-config-operator)
team. The helpers found here may be of use to you. They may not. They may
completely break entirely.

It is worth mentioning that these helpers may get your cluster into a
difficult-to-recover-from state. So do not use these on a production OpenShift
cluster.

## Installation

### GitHub Releases

Pre-built binaries for Linux and Mac on both amd64 and aarch64 architectures
are available for download via the [GitHub
Releases](https://github.com/cheesesashimi/zacks-openshift-helpers/releases)
page. All one has to do is download them and place them somewhere in your
`PATH`.

These binaries are built using [goreleaser](https://goreleaser.com/) running as
a GitHub Action. If you're using a Mac, you'll need to [jump through a few
hoops](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac)
to make these work for right now.

In the future, I plan to make each individual binary available for download in
addition to the current archive scheme.

### Containers

Starting with `v0.0.21`, you can `podman pull
quay.io/zzlotnik/zacks-openshift-helpers:latest` to get the latest version of
these binaries. Images are built for both AMD64 and ARM64 architectures. These
images also contain the latest stable versions of the `oc` and `kubectl`
commands since there are some portions of my helper binaries that shell out to
these commands in order to take the path of least resistance.

Full list of tags may be found [here](https://quay.io/repository/zzlotnik/zacks-openshift-helpers?tab=tags).

It is also worth noting that these binaries are also baked into the following
images as well, along with a few other of my favorite tools for working with
Kubernetes clusters:

- `quay.io/zzlotnik/toolbox:mco-fedora-39`
- `quay.io/zzlotnik/toolbox:mco-fedora-40`

*Note:* Although these images are rebuilt daily, there will be up to a 24-hour
delay between when the latest binaries are made available here and when they
are available in those images.

## Further Notes

- There is an `experimental` directory which contains purely experimental code. Use at your own risk.
- I purposely put all of my code under an `internal/` directory as I do not
  want this repository to be depended on for right now. However, that might change in the future.
- I have a cron job that periodically deletes recently-run GitHub Actions.
- As of `v0.0.20`, the binaries are no longer built with CGO enabled.
