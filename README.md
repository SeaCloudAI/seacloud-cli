<div align="center">
  <p>
    <img src="./assets/seacloud-cli-image-en.png" alt="SeaCloud CLI banner">
  </p>
  <h1>SeaCloud CLI</h1>
  <h3>The official CLI for the SeaCloud AI Platform</h3>
  <p>
    Built for AI agents. Authenticate, browse models, submit multimodal tasks,
    track task status, and manage SkillHub skills from any agent or terminal.
  </p>
  <p>
    <a href="https://www.npmjs.com/package/@seacloudai/seacloud-cli">
      <img src="https://img.shields.io/npm/v/@seacloudai/seacloud-cli" alt="npm version">
    </a>
    <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT License">
    <img src="https://img.shields.io/badge/node-%3E%3D18-339933" alt="Node.js >= 18">
    <img src="https://img.shields.io/badge/go-%3E%3D1.26-00ADD8" alt="Go >= 1.26">
  </p>
  <p>
    <a href="./README.zh.md">中文文档</a>
    ·
    <a href="https://vtrix.ai/">Official Website</a>
  </p>
</div>

## Features

- **Authentication**: Sign in with the browser-based device flow and store credentials locally.
- **Model discovery**: List available models and inspect full parameter specs in human-readable or JSON form.
- **Task execution**: Submit multimodal generation tasks from the CLI with parameter validation and structured output options.
- **Image generation via proxy**: Call sync image-generation models through `folkos-proxy`, with optional asset URL output.
- **Task tracking**: Poll task status and print result URLs or full JSON responses.
- **SkillHub integration**: Search, install, and configure agent skills from SeaCloud SkillHub.
- **Agent-friendly UX**: Supports `--dry-run`, JSON output, stable command shapes, and copy-pasteable examples.

## Install

### Install with npm

```bash
npm install -g @seacloudai/seacloud-cli
```

> Requires Node.js 18+

### Install from source

Default install:

```bash
git clone https://github.com/SeaCloudAI/seacloud-cli.git
cd seacloud-cli
make install
```

> Requires Go 1.26+
> The installed binary uses the default public service endpoints. In Folkos-managed runtimes, Vtrix generation requests are automatically rewritten to the fixed Folkos proxy URL.

If `/usr/local/bin` requires elevated permissions:

```bash
sudo make install
```

If you prefer a user-local install without `sudo`:

```bash
make install PREFIX=$HOME/.local
export PATH="$HOME/.local/bin:$PATH"
```

### Download binaries

Prebuilt binaries are published on the [Releases](https://github.com/SeaCloudAI/seacloud-cli/releases) page for:

- macOS `amd64`
- macOS `arm64`
- Linux `amd64`
- Linux `arm64`
- Windows `amd64`

## Quick Start

### Authenticate

```bash
seacloud auth login
seacloud auth status
```

### Browse models

```bash
seacloud models list
seacloud models list --type video
seacloud models spec kirin_v2_6_i2v
seacloud models spec kirin_v2_6_i2v --output json
```

### Run a task

```bash
seacloud run kirin_v2_6_i2v --param image=https://example.com/cat.jpg
seacloud run kirin_v2_6_i2v --param prompt="a cat running" --param duration=5
seacloud run kirin_v2_6_i2v --param mode=pro --output url
seacloud run gpt-image-2 --param prompt="a blue cat" --output url
```

### Generate an image through the proxy

```bash
SEACLOUD_FOLKOS_PROXY_URL=http://127.0.0.1:8090 seacloud images generate \
  --model gpt-image-2 \
  --prompt "A flat vector poster of a blue cat wearing black sunglasses" \
  --output json

SEACLOUD_FOLKOS_PROXY_URL=http://127.0.0.1:8090 seacloud images generate \
  --prompt "A flat vector poster of a blue cat wearing black sunglasses" \
  --output url
```

### Check task status

```bash
seacloud task status <task_id>
seacloud task status <task_id> --output url
seacloud task status <task_id> --output json
```

### Manage skills

```bash
seacloud skills list
seacloud skills find prompt
seacloud skills add some-skill
seacloud skills config --show
```

## Commands

### `seacloud auth`

```bash
seacloud auth login
seacloud auth status
seacloud auth logout
seacloud auth set-key <api-key>
```

### `seacloud models`

```bash
seacloud models list
seacloud models list --keywords kirin
seacloud models list --output id
seacloud models spec <model_id>
seacloud models spec <model_id> --output json
```

### `seacloud run`

```bash
seacloud run <model_id> --param key=value
seacloud run <model_id> --param prompt="hello" --param duration=5
seacloud run <model_id> --output json
seacloud run gpt-image-2 --param prompt="a blue cat" --output url
```

Nested fields use dot notation:

```bash
seacloud run some_model \
  --param camera_control.type=simple \
  --param camera_control.speed=2
```

### `seacloud task`

```bash
seacloud task status <task_id>
```

### `seacloud skills`

```bash
seacloud skills list
seacloud skills find [query]
seacloud skills add <slug>
seacloud skills config --show
```

### `seacloud images`

```bash
seacloud images generate --prompt="a blue cat"
seacloud images generate --prompt="a blue cat" --output json
seacloud images generate --prompt="a blue cat" --output url
```

### `seacloud version`

```bash
seacloud version
```

## Output and Automation

- Use `--output json` where supported for machine-readable responses.
- Use `--output url` on task commands to print only result URLs.
- Set `SEACLOUD_FOLKOS_PROXY_URL` to the root of your `folkos-proxy` service when using `seacloud images generate` or sync image models through `seacloud run`.
- Use global `--dry-run` to inspect execution without sending requests.

Example:

```bash
seacloud --dry-run run kirin_v2_6_i2v --param prompt=test
```

## Release

Release assets are built from source and published to GitHub Releases.  
The npm package downloads the matching prebuilt binary for the user platform during installation.

If you maintain releases manually, the repository includes:

- `scripts/build.sh`
- `.goreleaser.yml`
- `scripts/set-release-version.js`

## Repository Layout

```text
seacloud-cli/
├── cmd/                # CLI command definitions
├── internal/auth/      # Auth client and login flow
├── internal/models/    # Model list and spec APIs
├── internal/generation/# Task submit and polling
├── internal/skillhub/  # SkillHub client and install logic
├── package.json        # npm package manifest
├── scripts/            # Build, release, and npm wrapper scripts
└── skills/             # Built-in skill definitions
```

## Contributing

Issues and pull requests are welcome. Before sending larger changes, it is best to open an issue first so the scope can be discussed.

For local verification:

```bash
go test ./...
go run . --help
```
