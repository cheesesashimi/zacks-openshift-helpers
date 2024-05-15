#!/usr/bin/env python3

import os
import subprocess
import sys
import yaml


def create_build(name):
    return {
        "main": f"./cmd/{name}",
        "id": name,
        "binary": name,
        "env": [
            "CGO_ENABLED=0",
        ],
        "ldflags": [
            "-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=goreleaser".replace(
                "main",
                "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version",
            ),
        ],
        "goos": [
            "darwin",
            "linux",
        ],
        "goarch": [
            "amd64",
            "arm64",
        ],
    }


def update_goreleaser_file(names):
    goreleaser = ".goreleaser.yaml"

    with open(goreleaser, "r") as goreleaser_file:
        goreleaser_data = list(yaml.safe_load_all(goreleaser_file))[0]

    is_updated = False
    for name in names:
        new_build = create_build(name)
        if new_build in goreleaser_data["builds"]:
            continue

        print(f"Adding build for {new_build}")
        is_updated = True
        goreleaser_data["builds"].append(new_build)

    if not is_updated:
        print(f"{goreleaser} already up-to-date")
        return

    goreleaser_data["builds"].sort(key=lambda x: x["id"])
    with open(goreleaser, "w") as goreleaser_file:
        yaml.dump(goreleaser_data, goreleaser_file)


def update_gitignore_file(names):
    gitignore = ".gitignore"
    with open(gitignore, "r") as gitignore_file:
        gitignore_content = gitignore_file.read()

    is_updated = False
    with open(gitignore, "a") as gitignore_file:
        for name in names:
            cmd_name = f"cmd/{name}/{name}"
            if cmd_name not in gitignore_content:
                is_updated = True
                print(f"Added {cmd_name} to {gitignore}")
                gitignore_file.write(
                    f"# Auto-added by {os.path.basename(__file__)} to ignore ad-hoc Go binaries\n{cmd_name}\n"
                )

    if not is_updated:
        print(f"{gitignore} already up-to-date")
    else:
        print(f"{gitignore} updated")


def main():
    names = sorted(sys.argv[1:])

    update_goreleaser_file(names)

    update_gitignore_file(names)


if __name__ == "__main__":
    main()
