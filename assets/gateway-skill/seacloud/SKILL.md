---
name: seacloud
description: >-
  SeaCloud CLI is a multimodal task execution CLI designed specifically for
  Agents. With one SeaCloud API Key, it provides unified access to LLM, image,
  video, audio, 3D, and other models; supports model search, spec queries, task
  execution, and result tracking; and helps discover and manage professional
  skills for creative workflows through SkillHub. Use when the user asks to generate
  video, image, audio, music, 3D, run a SeaCloud model, inspect SeaCloud tasks,
  find/install agent skills, or automate SeaCloud workflows.
version: 0.0.18
allowed-tools: Bash(seacloud:*), Bash(npx seacloud:*), Bash(npx -y @seacloudai/seacloud-cli:*)
---

# seacloud

SeaCloud CLI is a multimodal task execution CLI designed specifically for
Agents. With one SeaCloud API Key, it provides unified access to LLM, image,
video, audio, 3D, and other models; supports model search, spec queries, task
execution, and result tracking; and helps discover and manage professional
skills for creative workflows through SkillHub.

Before running real SeaCloud commands, load the current CLI capabilities:

```bash
seacloud agent describe
```

Use that dynamic output as the source of truth for available commands,
parameters, output modes, authentication requirements, and recovery steps.
