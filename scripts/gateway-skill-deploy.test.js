const assert = require("assert");
const fs = require("fs");
const os = require("os");
const path = require("path");
const test = require("node:test");

const {
  compareSemver,
  deployGatewaySkill,
  readGatewaySkillVersion
} = require("./gateway-skill-deploy");

test("compareSemver orders x.y.z versions", () => {
  assert.equal(compareSemver("1.2.3", "1.2.3"), 0);
  assert.equal(compareSemver("1.2.4", "1.2.3"), 1);
  assert.equal(compareSemver("1.3.0", "1.2.9"), 1);
  assert.equal(compareSemver("2.0.0", "10.0.0"), -1);
});

test("deployGatewaySkill installs canonical skill and codex compatibility link", () => {
  const workspace = createWorkspace("0.0.18");
  fs.mkdirSync(path.join(workspace.homeDir, ".codex"), { recursive: true });

  const result = deployGatewaySkill({
    rootDir: workspace.rootDir,
    homeDir: workspace.homeDir,
    env: {},
    platform: "linux",
    logger: () => {}
  });

  const canonical = path.join(workspace.homeDir, ".agents", "skills", "seacloud", "SKILL.md");
  assert.equal(result.installed, true);
  assert.equal(readGatewaySkillVersion(canonical), "0.0.18");

  const codexTarget = path.join(workspace.homeDir, ".codex", "skills", "seacloud");
  assert.equal(fs.lstatSync(codexTarget).isSymbolicLink(), true);
  assert.equal(
    fs.realpathSync(codexTarget),
    fs.realpathSync(path.join(workspace.homeDir, ".agents", "skills", "seacloud"))
  );
});

test("deployGatewaySkill skips when SEACLOUD_SKIP_SKILL_DEPLOY is set", () => {
  const workspace = createWorkspace("0.0.18");

  const result = deployGatewaySkill({
    rootDir: workspace.rootDir,
    homeDir: workspace.homeDir,
    env: { SEACLOUD_SKIP_SKILL_DEPLOY: "1" },
    platform: "linux",
    logger: () => {}
  });

  assert.equal(result.skipped, true);
  assert.equal(fs.existsSync(path.join(workspace.homeDir, ".agents")), false);
});

test("deployGatewaySkill preserves same or newer canonical version", () => {
  const workspace = createWorkspace("0.0.18");
  const target = path.join(workspace.homeDir, ".agents", "skills", "seacloud");
  fs.mkdirSync(target, { recursive: true });
  fs.writeFileSync(path.join(target, "SKILL.md"), skillContent("9.0.0"), "utf8");

  const result = deployGatewaySkill({
    rootDir: workspace.rootDir,
    homeDir: workspace.homeDir,
    env: {},
    platform: "linux",
    logger: () => {}
  });

  assert.equal(result.installed, false);
  assert.equal(readGatewaySkillVersion(path.join(target, "SKILL.md")), "9.0.0");
});

test("deployGatewaySkill preserves newer compatibility target", () => {
  const workspace = createWorkspace("0.0.18");
  const target = path.join(workspace.homeDir, ".codex", "skills", "seacloud");
  fs.mkdirSync(target, { recursive: true });
  fs.writeFileSync(path.join(target, "SKILL.md"), skillContent("9.0.0"), "utf8");

  const result = deployGatewaySkill({
    rootDir: workspace.rootDir,
    homeDir: workspace.homeDir,
    env: {},
    platform: "linux",
    logger: () => {}
  });

  assert.equal(result.installed, true);
  assert.equal(result.linked, 0);
  assert.equal(readGatewaySkillVersion(path.join(target, "SKILL.md")), "9.0.0");
});

test("deployGatewaySkill overwrites older canonical version", () => {
  const workspace = createWorkspace("0.0.18");
  const target = path.join(workspace.homeDir, ".agents", "skills", "seacloud");
  fs.mkdirSync(target, { recursive: true });
  fs.writeFileSync(path.join(target, "SKILL.md"), skillContent("0.0.1"), "utf8");

  const result = deployGatewaySkill({
    rootDir: workspace.rootDir,
    homeDir: workspace.homeDir,
    env: {},
    platform: "linux",
    logger: () => {}
  });

  assert.equal(result.installed, true);
  assert.equal(readGatewaySkillVersion(path.join(target, "SKILL.md")), "0.0.18");
});

test("bundled gateway skill stays lightweight and points to dynamic guide", () => {
  const skillPath = path.join(__dirname, "..", "assets", "gateway-skill", "seacloud", "SKILL.md");
  const content = fs.readFileSync(skillPath, "utf8");

  assert.match(content, /^version:\s*[0-9]+\.[0-9]+\.[0-9]+$/m);
  assert.match(content, /^allowed-tools:\s*Bash\(seacloud:\*\)/m);
  assert.match(content, /seacloud agent describe/);
  assert.match(content, /seacloud account balance/);
});

function createWorkspace(version) {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), "seacloud-skill-deploy-test-"));
  const rootDir = path.join(base, "package");
  const homeDir = path.join(base, "home");
  const skillDir = path.join(rootDir, "assets", "gateway-skill", "seacloud");
  fs.mkdirSync(skillDir, { recursive: true });
  fs.mkdirSync(homeDir, { recursive: true });
  fs.writeFileSync(path.join(skillDir, "SKILL.md"), skillContent(version), "utf8");
  fs.writeFileSync(
    path.join(rootDir, "package.json"),
    JSON.stringify({ seacloud: { gatewaySkillDir: "assets/gateway-skill/seacloud" } }),
    "utf8"
  );
  return { base, rootDir, homeDir };
}

function skillContent(version) {
  return `---\nname: seacloud\nversion: ${version}\n---\n# seacloud\n`;
}
