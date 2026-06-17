const fs = require("fs");
const path = require("path");
const zlib = require("zlib");

function extractArchive(archivePath, destination, extension) {
  if (extension === "tar.gz") {
    extractTarGz(archivePath, destination);
    return;
  }
  if (extension === "zip") {
    extractZip(archivePath, destination);
    return;
  }
  throw new Error(`unsupported archive format: ${extension}`);
}

function extractTarGz(archivePath, destination) {
  const tarBuffer = zlib.gunzipSync(fs.readFileSync(archivePath));
  let offset = 0;

  while (offset + 512 <= tarBuffer.length) {
    const header = tarBuffer.subarray(offset, offset + 512);
    offset += 512;

    if (isZeroBlock(header)) {
      break;
    }

    const name = readTarName(header);
    const size = readOctal(header, 124, 12);
    const mode = readOctal(header, 100, 8);
    const type = String.fromCharCode(header[156] || 0);
    const body = tarBuffer.subarray(offset, offset + size);

    if (type === "0" || type === "\0" || type === "") {
      writeFile(destination, name, body, mode);
    } else if (type === "5") {
      fs.mkdirSync(resolveSafePath(destination, name), { recursive: true });
    }

    offset += Math.ceil(size / 512) * 512;
  }
}

function extractZip(archivePath, destination) {
  const buffer = fs.readFileSync(archivePath);
  const eocdOffset = findEndOfCentralDirectory(buffer);
  const entryCount = buffer.readUInt16LE(eocdOffset + 10);
  let centralOffset = buffer.readUInt32LE(eocdOffset + 16);

  for (let i = 0; i < entryCount; i += 1) {
    if (buffer.readUInt32LE(centralOffset) !== 0x02014b50) {
      throw new Error("invalid zip central directory");
    }

    const method = buffer.readUInt16LE(centralOffset + 10);
    const compressedSize = buffer.readUInt32LE(centralOffset + 20);
    const fileNameLength = buffer.readUInt16LE(centralOffset + 28);
    const extraLength = buffer.readUInt16LE(centralOffset + 30);
    const commentLength = buffer.readUInt16LE(centralOffset + 32);
    const localHeaderOffset = buffer.readUInt32LE(centralOffset + 42);
    const fileName = buffer
      .subarray(centralOffset + 46, centralOffset + 46 + fileNameLength)
      .toString("utf8");

    extractZipEntry(buffer, destination, {
      method,
      compressedSize,
      fileName,
      localHeaderOffset,
    });

    centralOffset += 46 + fileNameLength + extraLength + commentLength;
  }
}

function extractZipEntry(buffer, destination, entry) {
  if (buffer.readUInt32LE(entry.localHeaderOffset) !== 0x04034b50) {
    throw new Error(`invalid zip local header for ${entry.fileName}`);
  }

  const fileNameLength = buffer.readUInt16LE(entry.localHeaderOffset + 26);
  const extraLength = buffer.readUInt16LE(entry.localHeaderOffset + 28);
  const dataOffset = entry.localHeaderOffset + 30 + fileNameLength + extraLength;
  const compressed = buffer.subarray(dataOffset, dataOffset + entry.compressedSize);

  if (entry.fileName.endsWith("/")) {
    fs.mkdirSync(resolveSafePath(destination, entry.fileName), { recursive: true });
    return;
  }

  let body;
  if (entry.method === 0) {
    body = compressed;
  } else if (entry.method === 8) {
    body = zlib.inflateRawSync(compressed);
  } else {
    throw new Error(`unsupported zip compression method ${entry.method}`);
  }

  writeFile(destination, entry.fileName, body, 0o755);
}

function findEndOfCentralDirectory(buffer) {
  const minOffset = Math.max(0, buffer.length - 65557);
  for (let offset = buffer.length - 22; offset >= minOffset; offset -= 1) {
    if (buffer.readUInt32LE(offset) === 0x06054b50) {
      return offset;
    }
  }
  throw new Error("invalid zip: end of central directory not found");
}

function writeFile(destination, archiveName, body, mode) {
  const outputPath = resolveSafePath(destination, archiveName);
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, body);
  if (process.platform !== "win32" && mode) {
    fs.chmodSync(outputPath, mode & 0o777);
  }
}

function resolveSafePath(destination, archiveName) {
  if (
    archiveName.includes("\0") ||
    archiveName.startsWith("/") ||
    archiveName.startsWith("\\") ||
    /^[A-Za-z]:[\\/]/.test(archiveName)
  ) {
    throw new Error(`archive entry escapes destination: ${archiveName}`);
  }

  const normalized = archiveName.split("/").filter(Boolean).join(path.sep);
  const outputPath = path.resolve(destination, normalized);
  const root = path.resolve(destination);

  if (outputPath !== root && !outputPath.startsWith(`${root}${path.sep}`)) {
    throw new Error(`archive entry escapes destination: ${archiveName}`);
  }
  return outputPath;
}

function readTarName(header) {
  const name = readString(header, 0, 100);
  const prefix = readString(header, 345, 155);
  return prefix ? `${prefix}/${name}` : name;
}

function readString(buffer, offset, length) {
  const end = buffer.indexOf(0, offset);
  const sliceEnd = end === -1 || end > offset + length ? offset + length : end;
  return buffer.subarray(offset, sliceEnd).toString("utf8").trim();
}

function readOctal(buffer, offset, length) {
  const value = readString(buffer, offset, length).replace(/\0/g, "").trim();
  return value ? Number.parseInt(value, 8) : 0;
}

function isZeroBlock(buffer) {
  for (const byte of buffer) {
    if (byte !== 0) {
      return false;
    }
  }
  return true;
}

module.exports = { extractArchive };
