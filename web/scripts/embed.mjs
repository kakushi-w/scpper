import { cpSync, existsSync, rmSync, writeFileSync } from "node:fs";

const dist = new URL("../../internal/web/dist/", import.meta.url);
rmSync(dist, { recursive: true, force: true });
cpSync(new URL("../out/", import.meta.url), dist, { recursive: true });
if (!existsSync(new URL("index.html", dist))) throw new Error("the export has no index.html");
writeFileSync(new URL("README", dist), "Built pages. Run npm run build in web/ to refresh them.\n");
