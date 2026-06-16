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
    <a href="https://cloud.seaart.ai/">Official Website</a>
  </p>
</div>

## Features

- **Authentication**: Sign in with the browser-based device flow and store credentials locally.
- **Model discovery**: List available models and inspect full parameter specs in human-readable or JSON form.
- **Task execution**: Submit multimodal generation tasks from the CLI with parameter validation and structured output options.
- **Proxy-based image generation**: Call sync image-generation models through a compatible proxy service, with optional asset URL output.
- **Task tracking**: Poll task status and print result URLs or full JSON responses.
- **SkillHub integration**: Search, install, and configure agent skills from SeaCloud SkillHub.
- **Agent-friendly UX**: Supports `--dry-run`, JSON output, output limits, actionable errors, stable command shapes, and copy-pasteable examples.

## Install

### Install with npm

```bash
npm install -g @seacloudai/seacloud-cli
```

> Requires Node.js 18+

The npm installer also best-effort deploys a thin Gateway Skill to your agent
skills directory so new agent sessions can discover `seacloud`. Skill deployment
failures are reported as warnings and do not block the CLI binary install. Set
`SEACLOUD_SKIP_SKILL_DEPLOY=1` to skip this step.

Before an agent runs real SeaCloud commands, it should load the current CLI
capabilities from the installed binary:

```bash
seacloud agent describe
```

### Install from source

Default install:

```bash
git clone https://github.com/SeaCloudAI/seacloud-cli.git
cd seacloud-cli
make install
```

> Requires Go 1.26+
> The installed binary uses the default public service endpoints. In managed runtimes that inject `GATEWAY_URL` plus a managed token, Vtrix generation requests can be rewritten through the runtime proxy automatically.

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

### Check account balance

```bash
seacloud account balance
seacloud account balance --output json
```

### Browse models

```bash
seacloud models list
seacloud models spec kling_v2_6_i2v
seacloud models spec seedance_2_0 --output json
```

### Run a task

```bash
seacloud run kling_v2_6_i2v --param image=https://example.com/cat.jpg
seacloud run seedance_2_0 --param prompt="a cat running" --param duration=5
seacloud run kling_v2_6_i2v --param mode=pro --output url
seacloud run gpt-image-2 --param prompt="a blue cat" --output url
```

SeaCloud CLI accepts user-facing model IDs such as `kling_*`, `seedance_*`, and `seedream_*`.
They are resolved to the current backend IDs automatically before submission.

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

### Manage sandboxes

```bash
seacloud sandbox create base
seacloud sandbox create base --no-connect --wait
seacloud sandbox list --state running,paused --format json
seacloud sandbox exec <sandbox_id> ls -la
seacloud sandbox connect <sandbox_id>
seacloud sandbox kill <sandbox_id>
```

## Commands

### `seacloud auth`

```bash
seacloud auth login
seacloud auth status
seacloud auth logout
seacloud auth set-key <api-key>
```

### `seacloud account`

```bash
seacloud account balance
seacloud account balance --output json
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

### `seacloud sandbox`

Core sandbox commands follow the E2B CLI shape:

```bash
seacloud sandbox create [template]
seacloud sandbox create base --no-connect --wait
seacloud sandbox list --state running,paused --metadata app=agent --limit 10 --next-token <token>
seacloud sandbox exec <sandbox_id> "python --version"
seacloud sandbox exec --background <sandbox_id> "sleep 60 && echo done"
seacloud sandbox exec --cwd /workspace --user root --env NODE_ENV=production <sandbox_id> node app.js
seacloud sandbox connect <sandbox_id> --shell bash
seacloud sandbox kill <sandbox_id>
seacloud sandbox kill --all --state running,paused
seacloud sandbox metrics <sandbox_id>
seacloud sandbox metrics <sandbox_id_1> <sandbox_id_2> --output json
```

SeaCloud sandbox API features exposed by the SDKs are also available:

```bash
seacloud sandbox create base --auto-resume --allow-internet-access=false --allow-out 1.1.1.1 --volume-mount cache:/cache
seacloud sandbox network update <sandbox_id> --allow-public-traffic=true --deny-out 10.0.0.0/8
seacloud sandbox logs <sandbox_id> --limit 100 --direction backward
seacloud sandbox pause <sandbox_id>
seacloud sandbox timeout <sandbox_id> --seconds 3600
seacloud sandbox refresh <sandbox_id> --duration 300

seacloud sandbox volume create cache
seacloud sandbox volume list
seacloud sandbox volume get <volume_id>
seacloud sandbox volume delete <volume_id>

seacloud sandbox events --type sandbox.lifecycle.created
seacloud sandbox webhook create --name lifecycle --url https://example.com/webhook --secret whsec_... --event sandbox.lifecycle.created --max-attempts 5 --delay-seconds 1,5,30
seacloud sandbox webhook update <webhook_id> --enabled=false
seacloud sandbox webhook deliveries --status failed
seacloud sandbox webhook replay <delivery_id>

seacloud sandbox team list
seacloud sandbox team metrics <team_id> --start 1710000000 --end 1710003600
seacloud sandbox observability
```

`seacloud sandbox create <template>` follows E2B's interactive behavior when run in a terminal: it creates the sandbox, connects a shell, and kills the sandbox when the shell exits. Use `--no-connect` or `--output json` for automation.

### `seacloud template`

Template commands provide the E2B migration surface for local template projects and build operations:

```bash
seacloud template init --language typescript --name my-template
seacloud template migrate --language python --name my-template
seacloud template build my-template --dockerfile Dockerfile
seacloud template build my-template --image python:3.13 --cpu-count 2 --memory-mb 2048 --tag v1
seacloud template build my-template --from-template base --no-wait
seacloud template list --format json
seacloud template get my-template
seacloud template builds my-template
seacloud template status <template_id> <build_id>
seacloud template logs <template_id> <build_id> --limit 100
seacloud template tags assign my-template:v1 production stable
seacloud template tags list my-template
seacloud template tags remove my-template staging
seacloud template delete my-template
```

### `seacloud version`

```bash
seacloud version
```

## Output and Automation

- Use `--output json` where supported for machine-readable responses.
- Use `--format json` on sandbox/template commands for E2B-compatible output flag naming.
- Use `--output url` on task commands to print only result URLs.
- Set `SEACLOUD_FOLKOS_PROXY_URL` to the root of your proxy service when using `seacloud images generate` or sync image models through `seacloud run`.
- Set `SEACLOUD_SANDBOX_URL` or `SEACLOUD_BASE_URL` to the sandbox gateway API root. A gateway root such as `https://sandbox-gateway.cloud.seaart.ai` is normalized to `/api/v1`.
- Sandbox and template commands read API keys from the normal SeaCloud config, `SEACLOUD_API_KEY`, or E2B-compatible aliases `E2B_API_KEY` / `E2B_ACCESS_TOKEN`.
- Set `SEACLOUD_NAMESPACE_ID`, `SEACLOUD_USER_ID`, and `SEACLOUD_PROJECT_ID` when calling scoped sandbox APIs such as events, webhooks, volumes, teams, or metrics.
- Use global `--dry-run` before write/delete/replay operations. Dry-run output shows the method, path, body/query, destructive status, and the next step.
- Use `--limit`, `--next-token`, `--cursor`, or `--offset` on list/log/event commands to keep responses small.
- Parameter errors include the invalid field, what is wrong, and a suggested command or flag to fix it.

Example:

```bash
seacloud --dry-run run seedance_2_0 --param prompt=test
seacloud --dry-run sandbox webhook create --name lifecycle --url https://example.com/webhook --secret whsec_...
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
