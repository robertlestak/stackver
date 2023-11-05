# stackver

`stackver` is a tool to help track the versions of your stack dependencies. As systems become more microservice oriented, it is critical to stay on top of your version dependencies to ensure you are not running into any security vulnerabilities or functional regressions. While a CMDB is good at tracking the _historic_ versions of your stack, it is often difficult to relate these to their _current_ upstream versions.

`stackver` is a tool to help you do just that. It will take a list of your stack dependencies and utilize predefined upstream `trackers` to find any new releases. It will then compare these new releases to your current stack and output a report of any new releases, as well as notify you of if/when your current versions go EOL. This gives your technical teams visibility to and lead time on any upcoming changes to your stack, so they can be adeuqately integration tested in test environments before being deployed to production.

## Example Manifest

`stackver` is configured with a YAML (or JSON) manifest. This manifest should contain all of the dependencies in this stack, and their current versions. For example:

```yaml
---
metadata:
  name: stack
spec:
  dependencies:
  - name: kubernetes
    version: 1.27.3
  - name: istio
    version: 1.19.3
  - name: cert-manager
    version: 1.13.1
    tracker:
      kind: github
      uri: cert-manager/cert-manager
  - name: cert-manager-sync
    version: a22c122
    tracker:
      kind: github
      uri: robertlestak/cert-manager-sync
```

### Manifest Metadata

This contains the metadata for the manifest itself. Currently, the only required field is `name`, which is used to identify the manifest in the output. This is useful if you are tracking multiple stacks. `stackver` will update this will the current date/time when it is run.

### Manifest Spec

This contains the actual dependencies in your stack under the `dependencies` key. Each dependency should be defined by its `name` and `version`. The `name` is used to identify the dependency in the output, and the `version` is the current version of the dependency in your stack. This is the version that will be compared against the latest upstream version.

#### Depdenency

The `version` field is required and should be the current version of the dependency in your stack. This is the version that will be compared against the latest upstream version.

The `tracker` field is optional and should be used to define the upstream tracker for this dependency. If no tracker is defined, `stackver` will use the `endoflife` tracker by default. See the [Trackers](#trackers) section for more information.

If no `tracker.uri` is provided, the dependency key name is used.

## Trackers

`stackver` uses `trackers` to find new releases of your dependencies. A tracker is a simple Go interface that implements a `GetStatus` method. This method should retrieve the latest release details from the defined upstream, compare those against the currrent version, and return a `ServiceStatus` object. The following trackers are currently implemented:

### endoflife

The `endoflife` tracker uses the [endoflife.date](https://endoflife.date) API to track the EOL status of your dependencies. endoflife.date is an awesome community-driven project that tracks the EOL status of many popular open source projects. If no tracker is defined, this will be used as the default.

`endoflife` returns a full EOL date, enabling `stackver` to notify you of upcoming EOLs relative to the current date. This is the ideal tracker to use for most dependencies.

### github

The `github` tracker uses the [GitHub API](https://docs.github.com/en/rest) to track the latest releases of your dependencies. It will first try to find the version by the `/releases` API. If a release with the defined version is not found, it will fallback to searching by the commit hash in the `/commits` API.

Since `github` will only return the release itself and not meta information such as support cycles or EOL dates, `stackver` will not be able to notify you of upcoming EOLs. Instead, it will simply be able to notify you of new releases, so you will at least know when a new version is available.

*Note*: The GitHub API uses aggressive rate limits, so you'll probably want to set the `GITHUB_TOKEN` environment variable to a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token). This will increase your rate limit from 60 requests per hour to 5000 requests per hour.

## Outputs

When run, `stackver` evaulates the provided stack manifest and outputs a report of the current status of each dependency. The following output formats are currently supported:

### text

The `text` output format is the default output format. It will output a simple text report of the current status of each dependency. For example:

```text
Name               Version  Latest   EOL Date    Status   Link
kubernetes         1.27.3   1.28.3   2024-06-28  good     https://endoflife.date/kubernetes
istio              1.19.3   1.19.3   2024-03-31  current  https://endoflife.date/istio
cert-manager       1.13.1   1.13.2   unknown     good     https://github.com/cert-manager/cert-manager/releases
cert-manager-sync  a22c122  a22c122  unknown     current  https://github.com/robertlestak/cert-manager-sync
```

### yaml

The `yaml` output format will output a more detailed YAML report of the current status of each dependency. For example:

```yaml
---
metadata:
    name: stack
    lastChecked: 2023-11-05T09:24:37.201307-08:00
spec:
    dependencies:
      - name: kubernetes
        version: 1.27.3
        tracker:
            kind: endoflife
            uri: kubernetes
        status:
            latestVersion: 1.28.3
            currentVersionEOLDate: 2024-06-28T00:00:00Z
            currentVersionEOLIn: 5646h35m23.684825s
            link: https://endoflife.date/kubernetes
            status: good
      - name: istio
        version: 1.19.3
        tracker:
            kind: endoflife
            uri: istio
        status:
            latestVersion: 1.19.3
            currentVersionEOLDate: 2024-03-31T00:00:00Z
            currentVersionEOLIn: 3510h35m23.684845s
            link: https://endoflife.date/istio
            status: current
      - name: cert-manager
        version: 1.13.1
        tracker:
            kind: github
            uri: cert-manager/cert-manager
        status:
            latestVersion: 1.13.2
            link: https://github.com/cert-manager/cert-manager/releases
            status: good
      - name: cert-manager-sync
        version: a22c122
        tracker:
            kind: github
            uri: robertlestak/cert-manager-sync
        status:
            latestVersion: a22c122
            link: https://github.com/robertlestak/cert-manager-sync
            status: current
```

### json

The `json` output format will output a more detailed JSON report of the current status of each dependency, similar to YAML.

### prometheus

The `prometheus` output format will output a Prometheus Gauge metric which can be used to track the status of your stack dependencies over time. 

Status strings are converted to integers as follows (tl;dr lower is better):

- `current` = `0`
- `good` = `1`
- `update-available` = `2`
- `warning` = `3`
- `danger` = `4`
- `critical` = `5`

For example:

```text
# HELP stackver_service_status Stackver service status
# TYPE stackver_service_status gauge
stackver_service_status{eol_date="2024-03-31",latest="1.19.3",link="https://endoflife.date/istio",name="istio",status="current",version="1.19.3"} 0
stackver_service_status{eol_date="2024-06-28",latest="1.28.3",link="https://endoflife.date/kubernetes",name="kubernetes",status="good",version="1.27.3"} 1
stackver_service_status{eol_date="unknown",latest="1.13.2",link="https://github.com/cert-manager/cert-manager/releases",name="cert-manager",status="good",version="1.13.1"} 1
stackver_service_status{eol_date="unknown",latest="a22c122",link="https://github.com/robertlestak/cert-manager-sync",name="cert-manager-sync",status="current",version="a22c122"} 0
```

## Usage

```bash
Usage of stackver:
  -d int
        days until danger (default 30)
  -f string
        stack file
  -o string
        output format (default "text")
  -v    print version
  -w int
        days until warning (default 60)
```

`stackver` writes its output to `stdout`, so you can pipe it to a file or another program. For example:

```bash
$ stackver -f stack.yaml -o yaml > stack.yaml
```

If `-f` is a directory, `stackver` will recursively search for all `.yaml` and `.json` files and evaluate them all. When in directory mode, if an argument is passed, it will be used as the output directory. For example:

```bash
$ stackver -f stack -o yaml stack-reports
```

### Docker Usage

`stackver` is also available as a Docker image. You can run it as follows:

```bash
$ docker run --rm -v $(pwd):/stack robertlestak/stackver -f stack.yaml -o yaml > stack.yaml
```

The default working directory is `/stack`, so if you mount your stack manifests to this directory you can reference them with their relative paths as above. Otherwise you'll need to use the full path to the manifest, e.g. `-f /custom-mount/stack.yaml`.

## Automated Usage

`stackver` is designed to be run as part of an automated pipeline to periodicially check your dependencies and alert you of any upcoming EOL dates / new releases. `stackver` itself is solely responsible for generating the reports, and expects you to rely on pre-existing workflow and alerting systems. 

For example, you could run it as a GitHub Action on a schedule to track the versions of your stack dependencies over time and commit the reports to your repository. This would allow you to track the versions of your stack dependencies over time, as well as provide a historical record of the status of your stack. 

You could also use the `prometheus` output format and `node_exporter` to track the status of your stack dependencies over time in a Prometheus/Grafana dashboard, and alert you with Grafana's native alerting system.

### GitHub Action

`stackver` is published as a GitHub Action which you can use in your existing workflows. For example:

```yaml
name: stackver
on:
  # check on every push to main branch
  push:
    branches:
      - main
    # only check stack manifest changes
    paths:
      - 'stack-manifests/**'

  # check every day at midnight
  schedule:
    - cron: '0 0 * * *'

jobs:
  stackver:
    # this must be run on a linux machine
    runs-on: ubuntu-latest
    # let stackver access the repository
    permissions:
      contents: write
    steps:
    # checkout manifests
    - uses: actions/checkout@v4
    # run stackver and commit the reports to the repository
    - uses: robertlestak/stackver@main
      with:
        stack: stack-manifests
        output: reports/stack-manifests
        githubToken: ${{ secrets.GITHUB_TOKEN }}
```

This will run `stackver` on a schedule and commit the reports to the `reports/la0` directory in your repository. You can then use this to track the versions of your stack dependencies over time, as well as provide a historical record of the status of your stack.

Note that you must checkout your repository before running `stackver` so it can access your stack manifests. You must also provide a `githubToken` so `stackver` can access the GitHub API if you want to use the `github` tracker and/or push the reports back to your repository.