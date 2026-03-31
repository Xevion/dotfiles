#!/usr/bin/env bun

/**
 * bt-sync — Sync Bluetooth pairing keys from Windows to Linux.
 *
 * Reads pairing keys from a mounted Windows registry hive (via reged)
 * and compares/writes them to BlueZ's /var/lib/bluetooth/ config files.
 *
 * Usage:
 *   bt-sync --windows /media/xevion/Windows           # dry-run (default)
 *   bt-sync --windows /media/xevion/Windows --apply    # write changes
 *   bt-sync --windows /media/xevion/Windows --no-backup --apply
 */

import { existsSync, mkdirSync, readFileSync, writeFileSync, copyFileSync, readdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { parseArgs } from "node:util";
import { createInterface } from "node:readline";

interface WindowsDevice {
  mac: string; // uppercase colon-separated: "AC:BF:71:66:FE:B2"
  key: string; // uppercase hex: "48F826B9EE3EAB7B9494C951A5E47A33"
  isBLE: boolean;
}

interface WindowsAdapter {
  mac: string; // uppercase colon-separated
  devices: WindowsDevice[];
}

interface LinuxDevice {
  mac: string;
  name: string;
  linkKey: string | null;
  isBLE: boolean;
  infoPath: string;
  infoContent: string;
}

type DeviceStatus =
  | { kind: "match"; mac: string; name: string }
  | { kind: "differ"; mac: string; name: string; winKey: string; linuxKey: string }
  | { kind: "windows-only"; mac: string; winKey: string }
  | { kind: "linux-only"; mac: string; name: string }
  | { kind: "ble-skip"; mac: string; name: string };

function rawToColonMac(raw: string): string {
  // "bcc7469bd01e" → "BC:C7:46:9B:D0:1E"
  const upper = raw.toUpperCase();
  return upper.match(/.{2}/g)!.join(":");
}

function exportRegistryKeys(windowsPath: string): string {
  const hivePath = join(windowsPath, "Windows", "System32", "config", "SYSTEM");
  if (!existsSync(hivePath)) {
    console.error(`Error: Registry hive not found at ${hivePath}`);
    console.error("Is the Windows partition mounted correctly?");
    process.exit(1);
  }

  const tmpFile = `/tmp/bt-sync-export-${Date.now()}.reg`;
  const result = Bun.spawnSync([
    "reged",
    "-x",
    hivePath,
    String.raw`HKEY_LOCAL_MACHINE\SYSTEM`,
    String.raw`ControlSet001\Services\BTHPORT\Parameters\Keys`,
    tmpFile,
  ]);

  if (result.exitCode !== 0) {
    console.error("Error: reged export failed");
    console.error(result.stderr.toString());
    process.exit(1);
  }

  if (!existsSync(tmpFile)) {
    console.error("Error: reged produced no output file. Are there any BT keys in the registry?");
    process.exit(1);
  }

  const content = readFileSync(tmpFile, "utf-8");
  Bun.spawnSync(["rm", "-f", tmpFile]);
  return content;
}

const BLE_VALUE_NAMES = new Set(["ltk", "erand", "ediv", "irk", "csrk", "keylength", "centralirk"]);

function parseRegFile(rawContent: string): WindowsAdapter[] {
  const content = rawContent.replaceAll("\r", "");
  const adapters = new Map<string, WindowsDevice[]>();
  const bleAdapterDevices = new Set<string>();

  const sectionRe = /\[.*\\Keys\\([a-f0-9]{12})\]/i;
  const valueRe = /^"([^"]+)"=hex:(.+)$/;

  let currentAdapter: string | null = null;

  for (const line of content.split("\n")) {
    const sectionMatch = line.match(sectionRe);
    if (sectionMatch) {
      currentAdapter = sectionMatch[1].toLowerCase();
      if (!adapters.has(currentAdapter)) adapters.set(currentAdapter, []);
      continue;
    }

    if (!currentAdapter) continue;

    const valueMatch = line.match(valueRe);
    if (!valueMatch) continue;

    const [, valueName, hexData] = valueMatch;
    const nameLower = valueName.toLowerCase();

    // Check if this is a BLE-specific value name
    if (BLE_VALUE_NAMES.has(nameLower)) {
      // This device section contains BLE values — we'll mark it after parsing the MAC
      // BLE values are named (LTK, IRK, etc.) and live under a device MAC subkey
      // We need to track this differently — the section header is the adapter,
      // and BLE device MACs appear as subkey sections
      continue;
    }

    // Classic BT: value name is the device MAC (12 hex chars)
    if (/^[a-f0-9]{12}$/i.test(nameLower)) {
      const keyBytes = hexData
        .trim()
        .split(",")
        .map((b) => b.trim().toUpperCase().padStart(2, "0"))
        .join("");

      adapters.get(currentAdapter)!.push({
        mac: rawToColonMac(nameLower),
        key: keyBytes,
        isBLE: false,
      });
    }
  }

  // Now handle BLE device sections: [...\Keys\<adapter>\<device>]
  // These appear as sections with non-MAC value names (LTK, IRK, etc.)
  const bleDeviceSectionRe = /\[.*\\Keys\\([a-f0-9]{12})\\([a-f0-9]{12})\]/i;
  for (const line of content.split("\n")) {
    const match = line.match(bleDeviceSectionRe);
    if (match) {
      const adapterRaw = match[1].toLowerCase();
      const deviceRaw = match[2].toLowerCase();
      if (!adapters.has(adapterRaw)) adapters.set(adapterRaw, []);
      bleAdapterDevices.add(`${adapterRaw}:${deviceRaw}`);
    }
  }

  // Add BLE devices as entries
  for (const key of bleAdapterDevices) {
    const [adapterRaw, deviceRaw] = key.split(":");
    const devices = adapters.get(adapterRaw)!;
    // Don't duplicate if already added as classic
    if (!devices.some((d) => d.mac === rawToColonMac(deviceRaw))) {
      devices.push({
        mac: rawToColonMac(deviceRaw),
        key: "",
        isBLE: true,
      });
    }
  }

  return Array.from(adapters.entries()).map(([raw, devices]) => ({
    mac: rawToColonMac(raw),
    devices,
  }));
}

const BT_BASE = "/var/lib/bluetooth";

function readLinuxDevices(adapterMac: string): LinuxDevice[] {
  const adapterDir = join(BT_BASE, adapterMac);
  if (!existsSync(adapterDir)) return [];

  const devices: LinuxDevice[] = [];

  for (const entry of readdirSync(adapterDir)) {
    // Device directories look like "AC:BF:71:66:FE:B2"
    if (!/^[A-F0-9]{2}(:[A-F0-9]{2}){5}$/.test(entry)) continue;

    const infoPath = join(adapterDir, entry, "info");
    if (!existsSync(infoPath)) continue;

    const content = readFileSync(infoPath, "utf-8");
    const name = content.match(/^Name=(.+)$/m)?.[1] ?? "Unknown";
    const linkKey = content.match(/^\[LinkKey\]\nKey=([A-F0-9]{32})$/m)?.[1] ?? null;
    const isBLE = /^\[LongTermKey\]/m.test(content);

    devices.push({ mac: entry, name, linkKey, isBLE, infoPath, infoContent: content });
  }

  return devices;
}

function computeStatuses(winDevices: WindowsDevice[], linuxDevices: LinuxDevice[]): DeviceStatus[] {
  const statuses: DeviceStatus[] = [];
  const linuxByMac = new Map(linuxDevices.map((d) => [d.mac, d]));
  const seen = new Set<string>();

  for (const win of winDevices) {
    seen.add(win.mac);
    const linux = linuxByMac.get(win.mac);

    if (win.isBLE) {
      statuses.push({ kind: "ble-skip", mac: win.mac, name: linux?.name ?? "Unknown" });
      continue;
    }

    if (!linux) {
      statuses.push({ kind: "windows-only", mac: win.mac, winKey: win.key });
      continue;
    }

    if (linux.isBLE && !linux.linkKey) {
      statuses.push({ kind: "ble-skip", mac: win.mac, name: linux.name });
      continue;
    }

    if (linux.linkKey === win.key) {
      statuses.push({ kind: "match", mac: win.mac, name: linux.name });
    } else {
      statuses.push({
        kind: "differ",
        mac: win.mac,
        name: linux.name,
        winKey: win.key,
        linuxKey: linux.linkKey ?? "(none)",
      });
    }
  }

  for (const linux of linuxDevices) {
    if (!seen.has(linux.mac)) {
      statuses.push({ kind: "linux-only", mac: linux.mac, name: linux.name });
    }
  }

  return statuses;
}

function printStatuses(adapterMac: string, statuses: DeviceStatus[]) {
  console.log(`\nBluetooth Key Sync — Adapter ${adapterMac}\n`);

  if (statuses.length === 0) {
    console.log("  No devices found.\n");
    return;
  }

  for (const s of statuses) {
    switch (s.kind) {
      case "match":
        console.log(`  ${s.mac}  Keys match ✓`);
        if (s.name !== "Unknown") console.log(`  (${s.name})`);
        break;
      case "differ":
        console.log(`  ${s.mac}  Keys differ ⚠`);
        if (s.name !== "Unknown") console.log(`  (${s.name})`);
        console.log(`    Win:   ${s.winKey}`);
        console.log(`    Linux: ${s.linuxKey}`);
        break;
      case "windows-only":
        console.log(`  ${s.mac}  Windows only (new)`);
        break;
      case "linux-only":
        console.log(`  ${s.mac}  Linux only (no action)`);
        if (s.name !== "Unknown") console.log(`  (${s.name})`);
        break;
      case "ble-skip":
        console.log(`  ${s.mac}  BLE device — not yet supported ⚠`);
        if (s.name !== "Unknown") console.log(`  (${s.name})`);
        break;
    }
    console.log();
  }
}

function createPrompt(): { ask: (question: string) => Promise<string>; close: () => void } {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  return {
    ask: (question: string) => new Promise((resolve) => rl.question(question, resolve)),
    close: () => rl.close(),
  };
}

function backupInfoFile(infoPath: string, adapterMac: string, deviceMac: string) {
  const backupDir = join("/tmp", "bt-sync-backup", adapterMac, deviceMac);
  mkdirSync(backupDir, { recursive: true });
  const backupPath = join(backupDir, "info");
  copyFileSync(infoPath, backupPath);
  console.log(`  Backed up → ${backupPath}`);
}

function updateLinkKey(infoPath: string, currentContent: string, newKey: string) {
  // Replace the Key= line under [LinkKey]
  const updated = currentContent.replace(
    /(\[LinkKey\]\n)Key=[A-F0-9]+/,
    `$1Key=${newKey}`,
  );
  writeFileSync(infoPath, updated);
}

function createNewDeviceEntry(adapterMac: string, deviceMac: string, key: string) {
  const deviceDir = join(BT_BASE, adapterMac, deviceMac);
  mkdirSync(deviceDir, { recursive: true });
  const infoContent = `[General]
Name=Unknown
Trusted=true
Blocked=false

[LinkKey]
Key=${key}
Type=4
PINLength=0
`;
  writeFileSync(join(deviceDir, "info"), infoContent);
}

function ensureRoot() {
  if (process.getuid?.() === 0) return;

  console.log("Root access required to write bluetooth keys. Re-executing with sudo...\n");
  const result = Bun.spawnSync(["sudo", ...process.argv], {
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
  });
  process.exit(result.exitCode);
}

async function main() {
  const { values } = parseArgs({
    args: process.argv.slice(2),
    options: {
      windows: { type: "string", short: "w" },
      apply: { type: "boolean", default: false },
      "no-backup": { type: "boolean", default: false },
      help: { type: "boolean", short: "h", default: false },
    },
    strict: true,
  });

  if (values.help) {
    console.log(`bt-sync — Sync Bluetooth pairing keys from Windows to Linux

Usage:
  bt-sync --windows <path>              Show key differences (dry-run)
  bt-sync --windows <path> --apply      Write Windows keys to Linux

Options:
  -w, --windows <path>   Path to mounted Windows partition (required)
      --apply            Actually write keys (default is dry-run)
      --no-backup        Skip backing up existing info files
  -h, --help             Show this help`);
    process.exit(0);
  }

  if (!values.windows) {
    console.error("Error: --windows <path> is required");
    console.error("Example: bt-sync --windows /media/xevion/Windows");
    process.exit(1);
  }

  // Check reged is available
  const regedCheck = Bun.spawnSync(["which", "reged"]);
  if (regedCheck.exitCode !== 0) {
    console.error("Error: 'reged' not found. Install chntpw: sudo apt install chntpw");
    process.exit(1);
  }

  // Step 1: Export and parse Windows registry
  console.log("Reading Windows registry...");
  const regContent = exportRegistryKeys(values.windows);
  const adapters = parseRegFile(regContent);

  if (adapters.length === 0) {
    console.log("No Bluetooth adapters found in Windows registry.");
    process.exit(0);
  }

  // Reading /var/lib/bluetooth requires root even for dry-run
  ensureRoot();

  // Step 2: Read Linux devices and compute diff for each adapter
  let hasActionable = false;

  for (const adapter of adapters) {
    const linuxDevices = readLinuxDevices(adapter.mac);
    const statuses = computeStatuses(adapter.devices, linuxDevices);
    printStatuses(adapter.mac, statuses);

    const actionable = statuses.filter(
      (s) => s.kind === "differ" || s.kind === "windows-only",
    );

    if (actionable.length === 0) continue;
    hasActionable = true;

    if (!values.apply) {
      console.log("  Run with --apply to write changes.\n");
      continue;
    }

    // Interactive apply
    const prompt = createPrompt();
    const linuxByMac = new Map(linuxDevices.map((d) => [d.mac, d]));
    let anyWritten = false;

    for (const status of actionable) {
      if (status.kind === "differ") {
        const answer = await prompt.ask(
          `  ${status.mac} — Keys differ\n` +
            `    [w] Use Windows key  [l] Keep Linux key  [s] Skip\n    > `,
        );

        if (answer.toLowerCase() === "w") {
          const linux = linuxByMac.get(status.mac)!;
          if (!values["no-backup"]) backupInfoFile(linux.infoPath, adapter.mac, status.mac);
          updateLinkKey(linux.infoPath, linux.infoContent, status.winKey);
          console.log(`  ✓ Updated ${status.mac}\n`);
          anyWritten = true;
        } else {
          console.log(`  Skipped ${status.mac}\n`);
        }
      } else if (status.kind === "windows-only") {
        const answer = await prompt.ask(
          `  ${status.mac} — Not paired on Linux\n` +
            `    Create BlueZ entry with Windows key? [y/n]\n    > `,
        );

        if (answer.toLowerCase() === "y") {
          createNewDeviceEntry(adapter.mac, status.mac, status.winKey);
          console.log(`  ✓ Created entry for ${status.mac}\n`);
          anyWritten = true;
        } else {
          console.log(`  Skipped ${status.mac}\n`);
        }
      }
    }

    prompt.close();

    if (anyWritten) {
      const rl = createPrompt();
      const restart = await rl.ask(
        "Restart bluetooth.service now? Active connections will drop. [y/n] > ",
      );
      if (restart.toLowerCase() === "y") {
        console.log("Stopping bluetooth.service...");
        Bun.spawnSync(["systemctl", "stop", "bluetooth"]);
        console.log("Starting bluetooth.service...");
        Bun.spawnSync(["systemctl", "start", "bluetooth"]);
        console.log("✓ Bluetooth service restarted.\n");
      } else {
        console.log("Run 'sudo systemctl restart bluetooth' when ready.\n");
      }
      rl.close();
    }
  }

  if (!hasActionable) {
    console.log("All keys are in sync. Nothing to do.");
  }
}

main().catch((err) => {
  console.error("Fatal:", err.message);
  process.exit(1);
});
