import * as path from "node:path";
import Module from "node:module";

/**
 * Redirect `import "vscode"` to our local shim. Imported for side-effect
 * at the top of any test file that (transitively) loads src/*.ts modules.
 */
const shimPath = path.resolve(__dirname, "vscode-shim.ts");
const originalResolve = (Module as any)._resolveFilename;

if (!(Module as any).__teamcityPatched) {
  (Module as any)._resolveFilename = function (request: string, ...rest: any[]) {
    if (request === "vscode") return shimPath;
    return originalResolve.call(this, request, ...rest);
  };
  (Module as any).__teamcityPatched = true;
}

export { shimPath };
