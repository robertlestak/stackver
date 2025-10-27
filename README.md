# stackver

`stackver` is a dependabot-like tool that automatically tracks and updates versions of your stack dependencies. It can read versions directly from your deployment files (Kubernetes YAML, Helm charts, docker-compose, etc.), check for updates, and surgically update only the version numbers while preserving all formatting and comments.

As systems become more microservice oriented, it is critical to stay on top of your version dependencies to ensure you are not running into any security vulnerabilities or functional regressions. `stackver` automates this process by reading versions from your actual deployment files and updating them when new releases are available.

- [stackver](#stackver)
  - [Quick Start](#quick-start)
  - [Example Manifest](#example-manifest)
    - [Manifest Metadata](#manifest-metadata)
    - [Manifest Spec](#manifest-spec)
      - [Dependency](#dependency)
      - [Sources](#sources)
  - [Trackers](#trackers)
    - [endoflife](#endoflife)
    - [github](#github)
  - [Usage](#usage)
    - [Status Mode (Default)](#status-mode-default)
    - [Update Mode](#update-mode)
    - [Docker Usage](#docker-usage)
  - [Automated Usage](#automated-usage)
    - [GitHub Action](#github-action)
      - [Inputs](#inputs)

## Quick Start

1. **Create a stack manifest** pointing to your deployment files:
```yaml
---
metadata:
  name: my-stack
spec:
  dependencies:
  - name: nginx
    sources:
    - file: k8s/deployment.yaml
      selector: $.spec.template.spec.containers[0].image
    tracker:
      kind: endoflife
      uri: nginx
```

2. **Check for updates**:
```bash
stackver -f stack.yaml -dry-run
```

3. **Apply updates**:
```bash
stackver -f stack.yaml -update
```

## Example Manifest

`stackver` reads versions directly from your deployment files using JSONPath selectors:

```yaml
---
metadata:
  name: stack
spec:
  dependencies:
  - name: nginx
    sources:
    - file: k8s/deployment.yaml
      selector: $.spec.template.spec.containers[0].image
    - file: docker-compose.yml
      selector: $.services.web.image
    tracker:
      kind: endoflife
      uri: nginx
  - name: cert-manager
    sources:
    - file: helm/cert-manager/values.yaml
      selector: $.image.tag
    tracker:
      kind: github
      uri: cert-manager/cert-manager
```

### Manifest Metadata

This contains the metadata for the manifest itself. Currently, the only required field is `name`, which is used to identify the manifest in the output. This is useful if you are tracking multiple stacks.

### Manifest Spec

This contains the actual dependencies in your stack under the `dependencies` key. Each dependency should be defined by its `name` and `version`. The `name` is used to identify the dependency in the output, and the `version` is the current version of the dependency in your stack. This is the version that will be compared against the latest upstream version.

#### Dependency

Each dependency requires:

- `sources`: Array of file paths and selectors to read versions from
  - `file`: Path to YAML/JSON file containing the version
  - `selector`: JSONPath selector to extract the version (e.g., `$.spec.template.spec.containers[0].image`)

The `tracker` field is optional and defines the upstream tracker for this dependency. If no tracker is defined, `stackver` will use the `endoflife` tracker by default.

If no `tracker.uri` is provided, the dependency name is used.

#### Sources

Sources allow `stackver` to read versions directly from your deployment files:

```yaml
sources:
- file: k8s/deployment.yaml
  selector: $.spec.template.spec.containers[0].image
- file: helm/values.yaml
  selector: $.image.tag
```

Supported selector formats:
- JSONPath: `$.spec.containers[0].image`
- Nested keys: `$.image.tag`
- Array indexing: `$.containers[0].name`

## Trackers

`stackver` uses `trackers` to find new releases of your dependencies. A tracker is a simple Go interface that implements a `GetStatus` method. This method should retrieve the latest release details from the defined upstream, compare those against the currrent version, and return a `ServiceStatus` object. The following trackers are currently implemented:

### endoflife

The `endoflife` tracker uses the [endoflife.date](https://endoflife.date) API to track the EOL status of your dependencies. endoflife.date is an awesome community-driven project that tracks the EOL status of many popular open source projects. If no tracker is defined, this will be used as the default.

`endoflife` returns a full EOL date, enabling `stackver` to notify you of upcoming EOLs relative to the current date. This is the ideal tracker to use for most dependencies.

### github

The `github` tracker uses the [GitHub API](https://docs.github.com/en/rest) to track the latest releases of your dependencies. It will first try to find the version by the `/releases` API. If a release with the defined version is not found, it will fallback to searching by the commit hash in the `/commits` API.

Since `github` will only return the release itself and not meta information such as support cycles or EOL dates, `stackver` will not be able to notify you of upcoming EOLs. Instead, it will simply be able to notify you of new releases, so you will at least know when a new version is available.

*Note*: The GitHub API uses aggressive rate limits, so you'll probably want to set the `GITHUB_TOKEN` environment variable to a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token). This will increase your rate limit from 60 requests per hour to 5000 requests per hour.

## Usage

```bash
Usage of stackver:
  -d int
        days until danger (default 30)
  -f string
        stack file
  -update
        update files with new versions
  -dry-run
        show what would be updated without making changes
  -v    print version
  -w int
        days until warning (default 60)
```

### Status Mode (Default)

Show current status of dependencies:

```bash
stackver -f stack.yaml
```

### Update Mode

Dependabot-like functionality that can update your deployment files:

```bash
# Preview what would be updated (dry-run)
stackver -f stack.yaml -dry-run

# Apply updates to files
stackver -f stack.yaml -update
```

stackver will surgically update ONLY the version numbers while preserving all formatting, comments, and whitespace in your files.

### Docker Usage

`stackver` is also available as a Docker image. You can run it as follows:

```bash
$ docker run --rm -v $(pwd):/stack robertlestak/stackver -f stack.yaml -o yaml > stack.yaml
```

The default working directory is `/stack`, so if you mount your stack manifests to this directory you can reference them with their relative paths as above. Otherwise you'll need to use the full path to the manifest, e.g. `-f /custom-mount/stack.yaml`.

## Automated Usage

`stackver` is designed to be run as part of an automated pipeline to periodically check your dependencies and update your deployment files automatically, similar to dependabot.

For example, you could run it as a GitHub Action on a schedule to:
1. Check versions in your deployment files
2. Update them with new releases  
3. Create pull requests with the changes

The tool focuses on doing the work rather than generating reports - it either shows current status or updates your files.

### GitHub Action

`stackver` is published as a GitHub Action which you can use to automatically update dependencies and create pull requests. For example:

```yaml
name: Update Dependencies
on:
  # Check daily at 9 AM
  schedule:
    - cron: '0 9 * * *'
  
  # Allow manual trigger
  workflow_dispatch:

jobs:
  update-dependencies:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
    - uses: actions/checkout@v4
    
    - name: Update Dependencies
      uses: robertlestak/stackver@main
      with:
        stack: stack.yaml
        githubToken: ${{ secrets.GITHUB_TOKEN }}
        prTitle: "🔄 Update dependency versions"
        prBranch: "stackver/updates"
```

This will:
1. Check your stack manifest for version updates
2. Apply updates to your deployment files
3. Create a pull request with descriptive title including updated dependencies
4. Example PR titles: 
   - `🔄 Update dependency versions: nginx, redis`
   - `🔄 Update dependency versions: kubernetes`

#### Inputs

| Name | Description | Required | Default |
| --- | --- | --- | --- |
| `stack` | Path to your stack manifest file | `true` | N/A |
| `githubToken` | GitHub token for creating PRs | `true` | N/A |
| `dryRun` | Only show what would be updated | `false` | `false` |
| `prTitle` | Pull request title | `false` | `Update dependency versions` |
| `prBranch` | Branch name for pull request | `false` | `stackver/update-dependencies` |
| `daysUntilWarning` | Days until warning status | `false` | `60` |
| `daysUntilDanger` | Days until danger status | `false` | `30` |
| `stackVerVersion` | Version of stackver to use | `false` | `latest` |
