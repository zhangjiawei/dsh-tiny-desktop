// Native packaging is intentionally explicit: artifacts contain no developer
// profile, credentials, node_modules or downloaded user runtime.
import { spawnSync } from "node:child_process";
import { mkdir, writeFile, copyFile } from "node:fs/promises";
import sharp from "../frontend/node_modules/sharp/lib/index.js";
const version = process.env.VERSION || "0.3.1";
if (!/^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$/.test(version))
  throw Error("Invalid version");
const arch =
  process.env.GOARCH || { x64: "amd64", arm64: "arm64" }[process.arch];
const platform = process.platform;
const os = { darwin: "macos", win32: "windows", linux: "linux" }[platform];
function run(cmd, args) {
  const env = { ...process.env, GOARCH: arch };
  if (platform === "darwin") {
    env.MACOSX_DEPLOYMENT_TARGET = "13.0";
    env.CGO_CFLAGS = "-mmacosx-version-min=13.0";
    env.CGO_LDFLAGS = "-mmacosx-version-min=13.0";
  }
  const r = spawnSync(cmd, args, { stdio: "inherit", env });
  if (r.status !== 0) throw Error(`${cmd} failed: ${r.status}`);
}
await mkdir("dist", { recursive: true });
await mkdir("bin", { recursive: true });
const base = `dsh-tiny-desktop-${version}-${os}-${arch}`;
const icon = "frontend/dist/icon.png";
if (platform === "darwin") {
  const app = "dist/DSH Tiny.app",
    contents = app + "/Contents";
  await mkdir(contents + "/MacOS", { recursive: true });
  await mkdir(contents + "/Resources", { recursive: true });
  await mkdir("bin/AppIcon.iconset", { recursive: true });
  for (const size of [16, 32, 128, 256, 512]) {
    await sharp(icon)
      .resize(size)
      .toFile(`bin/AppIcon.iconset/icon_${size}x${size}.png`);
    await sharp(icon)
      .resize(size * 2)
      .toFile(`bin/AppIcon.iconset/icon_${size}x${size}@2x.png`);
  }
  run("iconutil", [
    "-c",
    "icns",
    "bin/AppIcon.iconset",
    "-o",
    contents + "/Resources/AppIcon.icns",
  ]);
  await writeFile(
    contents + "/Info.plist",
    `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>DSH Tiny</string>
<key>CFBundleDisplayName</key><string>DSH Tiny</string>
<key>CFBundleIdentifier</key><string>com.zhangjiawei.dsh-tiny-desktop</string>
<key>CFBundleExecutable</key><string>dsh-tiny</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>${version}</string>
<key>CFBundleVersion</key><string>${version}</string>
<key>CFBundleIconFile</key><string>AppIcon</string>
<key>LSMinimumSystemVersion</key><string>13.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSLocalNetworkUsageDescription</key><string>Connect to your local DSH workspace and optionally share it on a trusted private network.</string>
<key>NSAppTransportSecurity</key><dict>
<key>NSAllowsLocalNetworking</key><true/>
<key>NSAllowsArbitraryLoadsInWebContent</key><true/>
</dict></dict></plist>`,
  );
  run("go", [
    "build",
    "-tags",
    "production",
    "-trimpath",
    "-ldflags",
    `-s -w -X main.version=${version}`,
    "-o",
    contents + "/MacOS/dsh-tiny",
    "./cmd/desktop",
  ]);
  // Ad-hoc signing provides bundle integrity, not Apple notarization. README and
  // release notes must not imply a Developer ID signature is present.
  run("codesign", ["--force", "--deep", "--sign", "-", app]);
  run("codesign", ["--verify", "--deep", "--strict", app]);
  run("ditto", [
    "-c",
    "-k",
    "--sequesterRsrc",
    "--keepParent",
    app,
    `dist/${base}.zip`,
  ]);
} else {
  const dir = `dist/${base}`;
  await mkdir(dir, { recursive: true });
  const binary = "dsh-tiny" + (platform === "win32" ? ".exe" : "");
  if(platform==='win32') {
    // Embed the original icon and a per-monitor DPI-aware GUI manifest in the
    // executable itself, not only as a loose .ico next to it. No admin rights.
    run('go',['run','github.com/tc-hib/go-winres@v0.3.3','simply','--arch',arch,'--out','cmd/desktop/rsrc','--icon',icon,'--manifest','gui','--product-name','DSH Tiny','--file-description','DSH Tiny Desktop','--product-version',version,'--file-version',version]);
  }
  run("go", [
    "build",
    "-tags",
    "production",
    "-trimpath",
    "-ldflags",
    `-s -w -X main.version=${version}` +
      (platform === "win32" ? " -H windowsgui" : ""),
    "-o",
    `${dir}/${binary}`,
    "./cmd/desktop",
  ]);
  await copyFile(icon, dir + "/icon.png");
  await copyFile("README.md", dir + "/README.md");
  await copyFile("LICENSE", dir + "/LICENSE");
  if (platform === "win32") {
    const png = await sharp(icon).resize(256).png().toBuffer();
    const header = Buffer.alloc(22);
    header.writeUInt16LE(1, 2);
    header.writeUInt16LE(1, 4);
    header.writeUInt16LE(1, 10);
    header.writeUInt16LE(32, 12);
    header.writeUInt32LE(png.length, 14);
    header.writeUInt32LE(22, 18);
    await writeFile(dir + "/AppIcon.ico", Buffer.concat([header, png]));
    run("powershell", [
      "-NoProfile",
      "-Command",
      `Compress-Archive -Path '${dir}/*' -DestinationPath 'dist/${base}.zip' -Force`,
    ]);
  } else {
    await writeFile(
      dir + "/dsh-tiny.desktop",
      "[Desktop Entry]\nType=Application\nName=DSH Tiny\nExec=dsh-tiny\nIcon=dsh-tiny\nCategories=Development;\nTerminal=false\n",
    );
    run("tar", ["-czf", `dist/${base}.tar.gz`, "-C", "dist", base]);
  }
}
console.log("Packaged", base);
