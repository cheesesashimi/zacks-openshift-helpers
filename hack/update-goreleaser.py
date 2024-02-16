#!/usr/bin/env python3

import subprocess
import sys
import yaml

def create_build(name):
    return {
        "main": f"./cmd/{name}",
        "id": name,
        "binary": name,
        "goos": [
            "darwin",
            "linux",
        ],
        "goarch": [
            "amd64",
            "arm64",
        ],
    }

def main():
    names = sorted(sys.argv[1:])

    with open(".goreleaser.yaml", "r") as goreleaser_file:
        goreleaser_data = list(yaml.safe_load_all(goreleaser_file))[0]

    goreleaser_data["builds"]= [create_build(name) for name in names]

    with open(".goreleaser.yaml", "w") as goreleaser_file:
        yaml.dump(goreleaser_data, goreleaser_file)


if __name__ == "__main__":
    main()
