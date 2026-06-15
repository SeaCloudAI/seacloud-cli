const fs = require("fs");
const os = require("os");
const path = require("path");

function deployGatewaySkill(options = {}) {
  const rootDir = options.rootDir || path.resolve(__dirname, "..");
  const homeDir = options.homeDir || os.homedir();
  const env = options.env || process.env;
  const platform = options.platform || process.platform;
  const logger = options.logger || defaultLogger;
  const warnings = [];

  if (env.SEACLOUD_SKIP_SKILL_DEPLOY === "1") {
    logger("skip gateway skill deploy because SEACLOUD_SKIP_SKILL_DEPLOY=1");
    return { skipped: true, installed: false, linked: 0, warnings };
  }

  const sourceDir = resolveGatewaySkillDir(rootDir);
  if (!fs.existsSync(path.join(sourceDir, "SKILL.md"))) {
    warn(logger, warnings, `gateway skill source not found: ${sourceDir}`);
    return { skipped: false, installed: false, linked: 0, warnings };
  }

  const canonicalDir = path.join(homeDir, ".agents", "skills", "seacloud");
  const installed = installIfNewer(sourceDir, canonicalDir, logger, warnings);
  const linked = linkDetectedAgents(canonicalDir, homeDir, env, platform, logger, warnings);

  return { skipped: false, installed, linked, warnings };
}

function resolveGatewaySkillDir(rootDir) {
  const pkgPath = path.join(rootDir, "package.json");
  let configured = "assets/gateway-skill/seacloud";
  if (fs.existsSync(pkgPath)) {
    try {
      const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
      configured = pkg.seacloud?.gatewaySkillDir || configured;
    } catch (err) {
      // Keep the default path if package metadata is not readable.
    }
  }
  return path.join(rootDir, configured);
}

function installIfNewer(sourceDir, targetDir, logger, warnings) {
  const sourceSkill = path.join(sourceDir, "SKILL.md");
  const targetSkill = path.join(targetDir, "SKILL.md");
  const sourceVersion = readGatewaySkillVersion(sourceSkill);
  const targetVersion = readGatewaySkillVersion(targetSkill);

  if (targetVersion && sourceVersion && compareSemver(targetVersion, sourceVersion) >= 0) {
    logger(`gateway skill is already up to date at ${targetDir}`);
    return false;
  }
  if (fs.existsSync(targetSkill) && !targetVersion) {
    warn(logger, warnings, `existing gateway skill has no parseable version: ${targetSkill}`);
  }

  fs.rmSync(targetDir, { recursive: true, force: true });
  copyDir(sourceDir, targetDir);
  logger(`installed gateway skill to ${targetDir}`);
  return true;
}

function linkDetectedAgents(canonicalDir, homeDir, env, platform, logger, warnings) {
  let linked = 0;
  for (const agentDir of detectAgentSkillDirs(homeDir, env)) {
    if (path.resolve(agentDir) === path.resolve(path.dirname(canonicalDir))) {
      continue;
    }
    const targetDir = path.join(agentDir, "seacloud");
    try {
      fs.mkdirSync(agentDir, { recursive: true });
      if (targetIsCurrent(targetDir, canonicalDir)) {
        continue;
      }
      if (!shouldReplaceTarget(targetDir, canonicalDir, logger, warnings)) {
        continue;
      }
      fs.rmSync(targetDir, { recursive: true, force: true });
      if (platform === "win32") {
        copyDir(canonicalDir, targetDir);
      } else {
        fs.symlinkSync(canonicalDir, targetDir, "dir");
      }
      linked += 1;
      logger(`linked gateway skill to ${targetDir}`);
    } catch (err) {
      warn(logger, warnings, `failed to link gateway skill to ${targetDir}: ${err.message}`);
    }
  }
  return linked;
}

function shouldReplaceTarget(targetDir, canonicalDir, logger, warnings) {
  const targetSkill = path.join(targetDir, "SKILL.md");
  if (!fs.existsSync(targetDir)) {
    return true;
  }

  const targetVersion = readGatewaySkillVersion(targetSkill);
  const canonicalVersion = readGatewaySkillVersion(path.join(canonicalDir, "SKILL.md"));
  if (targetVersion && canonicalVersion && compareSemver(targetVersion, canonicalVersion) >= 0) {
    logger(`gateway skill is already up to date at ${targetDir}`);
    return false;
  }
  if (fs.existsSync(targetSkill) && !targetVersion) {
    warn(logger, warnings, `existing gateway skill has no parseable version: ${targetSkill}`);
  }
  return true;
}

function detectAgentSkillDirs(homeDir, env) {
  const dirs = [];
  const codexHome = env.CODEX_HOME || path.join(homeDir, ".codex");

  if (env.CODEX_HOME || exists(path.join(homeDir, ".codex"))) {
    dirs.push(path.join(codexHome, "skills"));
  }
  if (env.CURSOR_AGENT || exists(path.join(homeDir, ".cursor"))) {
    dirs.push(path.join(homeDir, ".cursor", "skills"));
  }
  if (pathContains(env.PATH, "claude-code") || exists(path.join(homeDir, ".claude"))) {
    dirs.push(path.join(homeDir, ".claude", "skills"));
  }
  if (exists(path.join(homeDir, ".continue"))) {
    dirs.push(path.join(homeDir, ".continue", "skills"));
  }
  if (exists(path.join(homeDir, ".openclaw"))) {
    dirs.push(path.join(homeDir, ".openclaw", "skills"));
  }
  return unique(dirs);
}

function targetIsCurrent(targetDir, canonicalDir) {
  if (!fs.existsSync(targetDir)) {
    return false;
  }
  try {
    return fs.realpathSync(targetDir) === fs.realpathSync(canonicalDir);
  } catch (err) {
    return false;
  }
}

function readGatewaySkillVersion(skillPath) {
  if (!skillPath || !fs.existsSync(skillPath)) {
    return "";
  }
  const text = fs.readFileSync(skillPath, "utf8");
  const match = text.match(/^version:\s*["']?([0-9]+\.[0-9]+\.[0-9]+)["']?\s*$/m);
  return match ? match[1] : "";
}

function compareSemver(a, b) {
  const left = parseSemver(a);
  const right = parseSemver(b);
  if (!left || !right) {
    return 0;
  }
  for (let i = 0; i < 3; i += 1) {
    if (left[i] > right[i]) return 1;
    if (left[i] < right[i]) return -1;
  }
  return 0;
}

function parseSemver(version) {
  const match = String(version || "").match(/^(\d+)\.(\d+)\.(\d+)$/);
  return match ? match.slice(1).map((part) => Number(part)) : null;
}

function copyDir(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const srcPath = path.join(src, entry.name);
    const dstPath = path.join(dst, entry.name);
    if (entry.isDirectory()) {
      copyDir(srcPath, dstPath);
    } else if (entry.isFile()) {
      fs.copyFileSync(srcPath, dstPath);
    }
  }
}

function exists(target) {
  return fs.existsSync(target);
}

function pathContains(pathValue, segment) {
  return String(pathValue || "").split(path.delimiter).some((part) => part.includes(segment));
}

function unique(values) {
  return Array.from(new Set(values));
}

function warn(logger, warnings, message) {
  warnings.push(message);
  logger(`warning: ${message}`);
}

function defaultLogger(message) {
  console.log(`[seacloud installer] ${message}`);
}

module.exports = {
  compareSemver,
  deployGatewaySkill,
  readGatewaySkillVersion
};
