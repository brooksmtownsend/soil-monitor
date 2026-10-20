# container-image-publish Specification

## Purpose
Define how GitHub Actions validates the project and publishes versioned container images to GitHub Container Registry for GitOps consumption.

## Requirements

### Requirement: Validation before publication
The workflow SHALL run Go tests and static analysis before pushing any image, and SHALL NOT push an image when they fail.

#### Scenario: Tests fail
- **WHEN** tests or vet fail in the workflow
- **THEN** no image is pushed

### Requirement: Pull requests do not publish
On pull requests the workflow SHALL validate and build the image without pushing it, and SHALL NOT require registry write credentials from forks.

#### Scenario: Pull request opened
- **WHEN** a pull request triggers the workflow
- **THEN** tests run and the image builds for both platforms, and no registry push occurs

### Requirement: Versioned push to GHCR
On pushes to the default branch and on version tags, the workflow SHALL push a multi-platform image to `ghcr.io/<owner>/<repository>` using the workflow's repository-scoped `GITHUB_TOKEN`, tagged with the commit SHA and, for version tags, the semantic version. A mutable `latest` tag SHALL NOT be required for deployment.

#### Scenario: Default-branch push
- **WHEN** a commit lands on the default branch and validation passes
- **THEN** an image tagged with the commit SHA is pushed

#### Scenario: Version tag
- **WHEN** a `vX.Y.Z` tag is pushed and validation passes
- **THEN** an image tagged `X.Y.Z` is pushed

### Requirement: Least-privilege workflow permissions
The workflow SHALL declare minimal permissions (read contents; write packages only for publishing) and SHALL NOT use long-lived registry credentials.

#### Scenario: Permissions are reviewed
- **WHEN** the workflow file is inspected
- **THEN** top-level permissions are read-only and package write is granted only to the publishing job

### Requirement: Image provenance
Published images SHALL carry provenance metadata and OCI labels identifying the source repository and revision.

#### Scenario: Image metadata is inspected
- **WHEN** a published image is inspected
- **THEN** its labels identify the source repository and commit
