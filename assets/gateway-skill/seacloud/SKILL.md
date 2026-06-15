---
name: seacloud
description: Use SeaCloud CLI for multimodal AI generation, model discovery, task tracking, SkillHub skill discovery/installation, and SeaCloud sandbox/template workflows. Use when the user asks to generate video, image, audio, music, 3D, run a SeaCloud model, inspect SeaCloud tasks, find/install agent skills, or automate SeaCloud workflows.
version: 0.0.18
allowed-tools: Bash(seacloud:*), Bash(npx seacloud:*), Bash(npx -y @seacloudai/seacloud-cli:*)
---

# seacloud

SeaCloud CLI is the entry point for SeaCloud multimodal generation, task
tracking, SkillHub, sandbox, and template workflows.

Before running real SeaCloud commands, load the current CLI capabilities:

```bash
seacloud agent describe
```

Use that dynamic output as the source of truth for available commands,
parameters, output modes, authentication requirements, and recovery steps.
