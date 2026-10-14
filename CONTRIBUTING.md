# Contributing

## Building and testing

- `make` formats and builds.
- `make check-all` runs the build, the unit tests, the live suites on both servers and the linters. It needs docker.
- A new tool needs a live test. The tool suite fails if a registered tool is never called.

The details are in [docs/development.md](docs/development.md).

## Changelog

A change a user would notice gets a line in [CHANGELOG.md](CHANGELOG.md) under `Unreleased`, in one of its sections: Breaking, Changed, Added or Fixed.

## Releases

- Changes collect under `Unreleased`.
- A version is tagged when a batch is worth upgrading for, not for every change.
- Breaking changes are grouped into one release, not shipped one per release.
- A release is cut from a commit where CI is green. The `Unreleased` heading becomes the version and its date, and the tag `vX.Y.Z` starts the release workflow. That publishes the binaries, the Docker image and the Homebrew formula.
