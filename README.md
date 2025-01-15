# ghe-reposec
Tool for verifying security controls in GitHub Enterprise repositories.

## Install

### Binary distributions

Binary distributions are available in the [releases] section.

### Install from source

Install the Lava command with `go install`.

```sh
go install github.com/adevinta/ghe-reposec
```

### Requirements

`ghe-reposec` requires [Lava] in order to run.

## Configuration

The `ghe-reposec` tool can be configured using environment variables. Below are the available configuration options:

### General Configuration

- `REPOSEC_LOG_LEVEL`: The log level (default: `info`). Possible values: `debug`, `info`, `warn`, `error`.
- `REPOSEC_LOG_OUTPUT`: The log output (default: `stdout`). Possible values: `stdout`, `stderr`.
- `REPOSEC_LOG_OUTPUT_FORMAT`: The log output format (default: `text`). Possible values: `text`, `json`.
- `REPOSEC_TARGET_ORG`: The target GitHub organization.
- `REPOSEC_OUTPUT_FILE`: The output file path (default: `/tmp/reposec.csv`).
- `REPOSEC_OUTPUT_FORMAT`: The output format (default: `csv`). Possible values: `csv`, `json`.

### GitHub Enterprise Configuration

- `REPOSEC_GHE_TOKEN`: The GitHub Enterprise token **(required)**.
- `REPOSEC_GHE_BASE_URL`: The GitHub Enterprise base URL **(required)**.
- `REPOSEC_GHE_CONCURRENCY`: The number of concurrent requests to GitHub Enterprise (default: `15`).
- `REPOSEC_GHE_REPOSITORY_SIZE_LIMIT`: The maximum repository size in KB (default: `3145728`).
- `REPOSEC_GHE_INCLUDE_ARCHIVED`: Include archived repositories (default: `false`).
- `REPOSEC_GHE_INCLUDE_EMPTY`: Include empty repositories (default: `false`).
- `REPOSEC_GHE_INCLUDE_FORKS`: Include forked repositories (default: `false`).
- `REPOSEC_GHE_INCLUDE_TEMPLATES`: Include template repositories (default: `false`).
- `REPOSEC_GHE_INCLUDE_DISABLED`: Include disabled repositories (default: `false`).

### Lava Configuration

- `REPOSEC_LAVA_CONCURRENCY`: The number of concurrent Lava scans (default: `10`).
- `REPOSEC_LAVA_BINARY_PATH`: The path to the Lava binary (default: `/usr/bin/lava`).
- `REPOSEC_LAVA_CHECK_IMAGE`: The Lava check image (default: `vulcansec/vulcan-repository-sctrl:a20516f-4aae88d`).


## Contributing

**We are not accepting external contributions at the moment.**

[Lava]: https://github.com/adevinta/lava
[releases]: https://github.com/adevinta/ghe-reposec/releases
