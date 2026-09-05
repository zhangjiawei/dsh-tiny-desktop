import { build } from "esbuild";
import { mkdir, copyFile } from "node:fs/promises";
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
await sharp(
  Buffer.from(
    '<svg xmlns="http://www.w3.org/2000/svg" width="44" height="44"><rect x="3" y="6" width="38" height="32" rx="5" fill="none" stroke="black" stroke-width="3"/><path d="M4 15h36M12 21l6 5-6 5m12 0h8" fill="none" stroke="black" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  ),
)
  .png()
  .toFile("dist/tray.png");
