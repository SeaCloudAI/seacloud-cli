const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const zlib = require("node:zlib");

const { extractArchive } = require("./archive-extract");

test("extractArchive extracts tar.gz files and directories", () => {
  const workspace = makeWorkspace();
  try {
    const archivePath = path.join(workspace, "sample.tar.gz");
    const destination = path.join(workspace, "out");
    fs.mkdirSync(destination);

    fs.writeFileSync(
      archivePath,
      makeTarGz([
        { name: "bin/", type: "5" },
        { name: "bin/seacloud", body: "hello from tar\n", mode: 0o755 },
      ])
    );

    extractArchive(archivePath, destination, "tar.gz");

    assert.equal(
      fs.readFileSync(path.join(destination, "bin", "seacloud"), "utf8"),
      "hello from tar\n"
    );
    if (process.platform !== "win32") {
      assert.equal(
        fs.statSync(path.join(destination, "bin", "seacloud")).mode & 0o777,
        0o755
      );
    }
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});

test("extractArchive extracts stored and deflated zip entries", () => {
  const workspace = makeWorkspace();
  try {
    const archivePath = path.join(workspace, "sample.zip");
    const destination = path.join(workspace, "out");
    fs.mkdirSync(destination);

    fs.writeFileSync(
      archivePath,
      makeZip([
        { name: "plain.txt", body: "stored\n", method: 0 },
        { name: "nested/deflated.txt", body: "deflated\n", method: 8 },
      ])
    );

    extractArchive(archivePath, destination, "zip");

    assert.equal(fs.readFileSync(path.join(destination, "plain.txt"), "utf8"), "stored\n");
    assert.equal(
      fs.readFileSync(path.join(destination, "nested", "deflated.txt"), "utf8"),
      "deflated\n"
    );
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});

test("extractArchive rejects archive entries that escape the destination", () => {
  const workspace = makeWorkspace();
  try {
    const destination = path.join(workspace, "out");
    fs.mkdirSync(destination);

    for (const entryName of ["../escape.txt", "/tmp/escape.txt", "C:\\escape.txt"]) {
      const archivePath = path.join(workspace, `${entryName.replace(/[^A-Za-z0-9]/g, "_")}.tar.gz`);
      fs.writeFileSync(archivePath, makeTarGz([{ name: entryName, body: "bad\n" }]));

      assert.throws(
        () => extractArchive(archivePath, destination, "tar.gz"),
        /archive entry escapes destination/
      );
    }

    assert.equal(fs.existsSync(path.join(workspace, "escape.txt")), false);
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});

test("extractArchive rejects unsupported formats", () => {
  const workspace = makeWorkspace();
  try {
    const archivePath = path.join(workspace, "sample.rar");
    const destination = path.join(workspace, "out");
    fs.mkdirSync(destination);
    fs.writeFileSync(archivePath, "not an archive");

    assert.throws(
      () => extractArchive(archivePath, destination, "rar"),
      /unsupported archive format: rar/
    );
  } finally {
    fs.rmSync(workspace, { recursive: true, force: true });
  }
});

function makeWorkspace() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "seacloud-archive-test-"));
}

function makeTarGz(entries) {
  const blocks = [];
  for (const entry of entries) {
    const body = Buffer.from(entry.body || "");
    const header = Buffer.alloc(512);
    writeString(header, 0, 100, entry.name);
    writeOctal(header, 100, 8, entry.mode || 0o755);
    writeOctal(header, 108, 8, 0);
    writeOctal(header, 116, 8, 0);
    writeOctal(header, 124, 12, body.length);
    writeOctal(header, 136, 12, 0);
    header.fill(" ", 148, 156);
    header[156] = (entry.type || "0").charCodeAt(0);
    writeString(header, 257, 6, "ustar");
    writeString(header, 263, 2, "00");

    const checksum = header.reduce((sum, byte) => sum + byte, 0);
    writeOctal(header, 148, 8, checksum);

    blocks.push(header);
    if (body.length > 0) {
      blocks.push(body);
      blocks.push(Buffer.alloc((512 - (body.length % 512)) % 512));
    }
  }
  blocks.push(Buffer.alloc(1024));
  return zlib.gzipSync(Buffer.concat(blocks));
}

function writeString(buffer, offset, length, value) {
  buffer.write(value, offset, Math.min(length, Buffer.byteLength(value)), "utf8");
}

function writeOctal(buffer, offset, length, value) {
  const text = value.toString(8).padStart(length - 1, "0");
  buffer.write(text, offset, length - 1, "ascii");
  buffer[offset + length - 1] = 0;
}

function makeZip(entries) {
  const localParts = [];
  const centralParts = [];
  let localOffset = 0;

  for (const entry of entries) {
    const name = Buffer.from(entry.name, "utf8");
    const body = Buffer.from(entry.body, "utf8");
    const compressed = entry.method === 8 ? zlib.deflateRawSync(body) : body;

    const localHeader = Buffer.alloc(30);
    localHeader.writeUInt32LE(0x04034b50, 0);
    localHeader.writeUInt16LE(20, 4);
    localHeader.writeUInt16LE(entry.method, 8);
    localHeader.writeUInt32LE(0, 14);
    localHeader.writeUInt32LE(compressed.length, 18);
    localHeader.writeUInt32LE(body.length, 22);
    localHeader.writeUInt16LE(name.length, 26);

    localParts.push(localHeader, name, compressed);

    const centralHeader = Buffer.alloc(46);
    centralHeader.writeUInt32LE(0x02014b50, 0);
    centralHeader.writeUInt16LE(20, 4);
    centralHeader.writeUInt16LE(20, 6);
    centralHeader.writeUInt16LE(entry.method, 10);
    centralHeader.writeUInt32LE(0, 16);
    centralHeader.writeUInt32LE(compressed.length, 20);
    centralHeader.writeUInt32LE(body.length, 24);
    centralHeader.writeUInt16LE(name.length, 28);
    centralHeader.writeUInt32LE(localOffset, 42);
    centralParts.push(centralHeader, name);

    localOffset += localHeader.length + name.length + compressed.length;
  }

  const centralOffset = localOffset;
  const centralDirectory = Buffer.concat(centralParts);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(centralDirectory.length, 12);
  end.writeUInt32LE(centralOffset, 16);

  return Buffer.concat([...localParts, centralDirectory, end]);
}
