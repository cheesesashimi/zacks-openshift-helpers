#!/usr/bin/env python3

import itertools
import os
import subprocess
import sys
import yaml

DOCKERFILE_NAME = "Dockerfile.goreleaser"


# See: https://github.com/yaml/pyyaml/issues/535
class VerboseSafeDumper(yaml.SafeDumper):
    def ignore_aliases(self, data):
        return True


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


def create_docker_for_arch(builds, arch):
    return {
        "image_templates": [
            # Using a formatstring here doesn't work because of the nested {{}}'s.'
            "quay.io/zzlotnik/{{ .ProjectName }}:{{ .Version }}-"
            + arch,
        ],
        "dockerfile": DOCKERFILE_NAME,
        "goos": "linux",
        "goarch": arch,
        "ids": [build["id"] for build in builds],
        "build_flag_templates": [
            f"--platform=linux/{arch}",
            "--label=org.opencontainers.image.title={{ .ProjectName }}",
            "--label=org.opencontainers.image.description={{ .ProjectName }}",
            "--label=org.opencontainers.image.url=https://github.com/cheesesashimi/{{ .ProjectName }}",
            "--label=org.opencontainers.image.source=https://github.com/cheesesashimi/{{ .ProjectName }}",
            "--label=org.opencontainers.image.version={{ .Version }}",
            '--label=org.opencontainers.image.created={{ time "2006-01-02T15:04:05Z07:00" }}',
            "--label=org.opencontainers.image.revision={{ .FullCommit }}",
        ],
    }


def create_docker_manifests_for_dockers(dockers):
    image_templates = list(
        itertools.chain.from_iterable([docker["image_templates"] for docker in dockers])
    )

    return [
        {
            "name_template": "quay.io/zzlotnik/{{ .ProjectName }}:{{ .Version }}",
            "image_templates": image_templates,
        },
        {
            "name_template": "quay.io/zzlotnik/{{ .ProjectName }}:latest",
            "image_templates": image_templates,
        },
    ]


def update_goreleaser_file(names):
    goreleaser = ".goreleaser.yaml"

    with open(goreleaser, "r") as goreleaser_file:
        goreleaser_data = list(yaml.safe_load_all(goreleaser_file))[0]

    goreleaser_data["builds"] = [create_build(name) for name in names]
    goreleaser_data["builds"].sort(key=lambda x: x["id"])

    goreleaser_data["dockers"] = [
        create_docker_for_arch(goreleaser_data["builds"], arch)
        for arch in ["amd64", "arm64"]
    ]
    goreleaser_data["docker_manifests"] = create_docker_manifests_for_dockers(
        goreleaser_data["dockers"]
    )

    with open(goreleaser, "w") as goreleaser_file:
        yaml.dump(goreleaser_data, goreleaser_file, Dumper=VerboseSafeDumper)


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


def update_dockerfile(names):
    print(f"Regenerating {DOCKERFILE_NAME}")

    dockerfile_lines = [
        "# DO NOT EDIT BY HAND!",
        "# Auto-generated by hack/update-configs.py",
        "",
    ]

    with open("Dockerfile.fetcher", "r") as fetcher:
        for line in fetcher:
            dockerfile_lines.append(line.rstrip())

    dockerfile_lines.append("")
    dockerfile_lines.append("FROM quay.io/fedora/fedora:40 AS final")
    dockerfile_lines.append("COPY --from=fetcher /oc/oc /usr/local/bin/oc")
    dockerfile_lines.append("COPY --from=fetcher /oc/kubectl /usr/local/bin/kubectl")

    for name in names:
        dockerfile_lines.append(f"COPY {name} /usr/local/bin/{name}")

    with open(DOCKERFILE_NAME, "w") as dockerfile:
        for line in dockerfile_lines:
            dockerfile.write(f"{line}\n")


def main():
    names = sorted(sys.argv[1:])

    ignored_names = frozenset(["mcdiff"])

    names = [name for name in names if name not in ignored_names]

    update_goreleaser_file(names)

    update_gitignore_file(names)

    update_dockerfile(names)


if __name__ == "__main__":
    main()
