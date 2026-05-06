// scripts/find-archive-marker-bug.js — locate why the archive marker
// is not being written even though the tier classifies as Archived.
//
// The diagnostic output showed:
//   tier:       Archived  (computed from age: currentBlock - lastTouched > 1000)
//   isArchived: false     (read from rawdb.ReadArchiveMarker)
//
// This means RecordBlockTouches wrote the address into the block-touched
// list, but the sweep at currentBlock-ColdBlocks-1 did NOT mark it
// archived. Root causes (in order of likelihood):
//
//   1. Sweep iterates the wrong block. Its candidate block computation
//      may differ from the block where addr was originally indexed.
//
//   2. Sweep did run but threshold check was wrong: e.g. it requires
//      `lastTouched <= candidateBlock` when it should be `lastTouched
//      < currentBlock - ColdBlocks` etc.
//
//   3. WriteArchiveMarker is called but the value is later overwritten
//      by another path (e.g. RecordBlockTouches accidentally clears it).
//
//   4. The sweep's `MaxLifecycleSweepPerBlock` cap was hit and our addr
//      fell off the suffix every block — never sweeped.
//
//   5. The archive marker prefix byte differs between writer and reader.
//
// USAGE: requires REPO_ROOT in .env

const fs = require("fs");
const path = require("path");
const sh = require("./shared");

const REPO_ROOT = process.env.REPO_ROOT || "";
if (!REPO_ROOT || !fs.existsSync(REPO_ROOT)) {
  console.log("ERROR: REPO_ROOT not set in .env");
  process.exit(1);
}

console.log(`\n====== Archive Marker Bug Locator ======\n`);

function* walk(dir, depth = 0) {
  if (depth > 5) return;
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch (e) { return; }
  for (const e of entries) {
    if (e.name === "node_modules" || e.name === ".git" || e.name === "vendor") continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) yield* walk(p, depth + 1);
    else if (e.name.endsWith(".go")) yield p;
  }
}

function scan(label, regex, dir = REPO_ROOT, max = 50) {
  console.log(`--- ${label} ---`);
  let found = 0;
  for (const file of walk(dir)) {
    let lines;
    try { lines = fs.readFileSync(file, "utf8").split("\n"); } catch (e) { continue; }
    for (let i = 0; i < lines.length; i++) {
      if (regex.test(lines[i])) {
        const rel = path.relative(REPO_ROOT, file);
        console.log(`  ${rel}:${i + 1}  ${lines[i].trim().substring(0, 180)}`);
        found++;
        if (found >= max) return;
      }
    }
  }
  if (found === 0) console.log(`  (none found)`);
  console.log();
}

scan("1. WriteArchiveMarker / DeleteArchiveMarker definitions",
  /^func\s+(Write|Delete|Read)ArchiveMarker/);

scan("2. ProcessLifecycle / sweep entry",
  /^func\s+\(\w+\s+\*StateLifecycleEngine\)\s+(ProcessLifecycle|sweep|Sweep)/);

scan("3. RecordBlockTouches definition",
  /^func\s+\(\w+\s+\*StateLifecycleEngine\)\s+RecordBlockTouches/);

scan("4. Schema prefix bytes (T/C/W/L/X/x)",
  /(prefixArchive|prefixT|markerPrefix|0x[0-9A-Fa-f]{2}\s*\/\/\s*archive)/i);

scan("5. ReadBlockTouchedAddresses",
  /ReadBlockTouchedAddresses|WriteBlockTouchedAddresses/);

scan("6. Hook integration in lyra2 + ethash",
  /runStateLifecycle/);

scan("7. ComputeTier function",
  /^func\s+ComputeTier/);

scan("8. WriteLastTouched callers (where touch is recorded)",
  /WriteLastTouched|RecordBlockTouches/);

console.log(`====== END OF SCAN ======`);
console.log(`\nPaste output to Claude — focus on sections 1, 2, 7, and the`);
console.log(`prefix byte definitions in section 4. We need to verify:`);
console.log(`  (a) ProcessLifecycle correctly calculates candidate block`);
console.log(`  (b) ComputeTier returns Archived ONLY when marker is set`);
console.log(`      (currently it seems to return Archived from age alone)`);
console.log(``);
