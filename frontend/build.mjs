import { build } from "esbuild";
import { mkdir, copyFile, writeFile } from "node:fs/promises";
import sharp from "sharp";
await mkdir("dist", { recursive: true });
await build({
  entryPoints: ["src/main.ts"],
  bundle: true,
  format: "esm",
  minify: true,
  outfile: "dist/main.js",
  external: ["/wails/runtime.js"],
});
for (const f of ["index.html", "style.css", "icon.svg", "settings.svg"])
  await copyFile("src/" + f, "dist/" + f);
await sharp("src/icon.svg").png().toFile("dist/icon.png");
await sharp("src/settings.svg").png().toFile("dist/settings.png");
// A PNG-compressed ICO gives Windows an independent settings-window icon.
const gear = await sharp("src/settings.svg").resize(256).png().toBuffer();
const ico = Buffer.alloc(22);
ico.writeUInt16LE(1, 2); ico.writeUInt16LE(1, 4);
ico.writeUInt16LE(1, 10); ico.writeUInt16LE(32, 12);
ico.writeUInt32LE(gear.length, 14); ico.writeUInt32LE(22, 18);
await writeFile("dist/settings.ico", Buffer.concat([ico, gear]));
await sharp(
  Buffer.from(
    '<svg xmlns="http://www.w3.org/2000/svg" width="44" height="44"><rect x="3" y="6" width="38" height="32" rx="5" fill="none" stroke="black" stroke-width="3"/><path d="M4 15h36M12 21l6 5-6 5m12 0h8" fill="none" stroke="black" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  ),
)
  .png()
  .toFile("dist/tray.png");
