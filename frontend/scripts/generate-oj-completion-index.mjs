import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const profilePath = join(root, "oj-completion-profile.json");
const outputPath = join(root, "src", "generated", "oj-completion-index.json");
const profile = JSON.parse(readFileSync(profilePath, "utf8"));
const commands = {
  javap: process.env.OJ_INDEX_JAVAP || "javap",
  go: process.env.OJ_INDEX_GO || "go",
  python: process.env.OJ_INDEX_PYTHON || firstAvailableCommand(["python3.13", "python3"]),
  clang: process.env.OJ_INDEX_CLANG || "clang++"
};

const index = {
  schema_version: "oj.completion.index.v1",
  profile_schema_version: profile.schema_version,
  judge_targets: profile.judge_targets,
  generation_toolchains: {
    java: commandVersion(commands.javap, ["-version"]),
    go: commandVersion(commands.go, ["version"]),
    python: commandVersion(commands.python, ["--version"]),
    javascript: `TypeScript ${ts.version}`,
    typescript: `TypeScript ${ts.version}`,
    cpp: commandVersion(commands.clang, ["--version"])
  },
  languages: {}
};

index.languages.java = generateJava(profile.languages.java);
index.languages.go = generateGo(profile.languages.go);
index.languages.python = generatePython(profile.languages.python);
const ecmaIndex = generateTypeScript(profile.languages.javascript);
index.languages.javascript = ecmaIndex;
index.languages.typescript = inheritLanguage(ecmaIndex, profile.languages.typescript);
index.languages.cpp = generateCpp(profile.languages.cpp);

const output = `${JSON.stringify(index, null, 2)}\n`;
if (process.argv.includes("--check")) {
  if (readFileSync(outputPath, "utf8") !== output) {
    throw new Error("generated OJ completion index is stale; run npm run generate:oj-index");
  }
} else {
  mkdirSync(dirname(outputPath), { recursive: true });
  writeFileSync(outputPath, output);
  process.stdout.write(`generated ${outputPath}\n`);
}

function generateJava(languageProfile) {
  const types = {};
  for (const [name, config] of Object.entries(languageProfile.types)) {
    const text = execFileSync(commands.javap, ["-public", config.probe], { encoding: "utf8" });
    const members = [];
    for (const rawLine of text.split("\n")) {
      const line = rawLine.trim();
      if (!line.endsWith(";") || !line.includes("(")) continue;
      const isStatic = /\bstatic\b/.test(line);
      if ((config.mode === "static") !== isStatic) continue;
      const match = line.match(/([A-Za-z_$][\w$]*)\((.*)\)(?:\s+throws\s+.*)?;$/);
      if (!match || match[1] === config.probe.split(".").at(-1)) continue;
      members.push(member(name, match[1], cleanJavaDetail(line), splitTopLevel(match[2]).length));
    }
    types[name] = typeEntry(config, members);
  }
  return { types };
}

function generateGo(languageProfile) {
  const types = {};
  for (const [name, config] of Object.entries(languageProfile.types)) {
    const text = execFileSync(commands.go, ["doc", "-all", config.probe], { encoding: "utf8" });
    const members = [];
    for (const rawLine of text.split("\n")) {
      const line = rawLine.trim();
      if (!line.startsWith("func ")) continue;
      const declaration = config.mode === "type"
        ? line.replace(/^func\s+\([^)]+\)\s+/, "")
        : line.replace(/^func\s+/, "");
      const match = declaration.match(/^([A-Z]\w*)\(/);
      if (!match) continue;
      const openParen = declaration.indexOf("(");
      const closeParen = matchingParen(declaration, openParen);
      if (closeParen < 0) continue;
      const parameters = declaration.slice(openParen + 1, closeParen);
      members.push(member(name, match[1], line, splitTopLevel(parameters).length));
    }
    types[name] = typeEntry(config, members);
  }
  return { types };
}

function generatePython(languageProfile) {
  const script = `
import importlib, inspect, json, sys
profile = json.loads(sys.argv[1])
result = {}
for name, config in profile["types"].items():
    module_name, _, attr_path = config["probe"].partition(".")
    target = importlib.import_module(module_name)
    if attr_path:
        for part in attr_path.split("."):
            target = getattr(target, part)
    members = []
    for label, value in inspect.getmembers(target):
        if label.startswith("_"):
            continue
        if config.get("mode") != "module" and not (callable(value) or isinstance(value, property)):
            continue
        if config.get("mode") == "module" and not callable(value):
            continue
        try:
            signature_value = inspect.signature(value)
            signature = str(signature_value)
            arg_count = max(0, len(signature_value.parameters) - (0 if config.get("mode") == "module" else 1)) if callable(value) else 0
        except (TypeError, ValueError):
            signature = "()" if callable(value) else ""
            arg_count = 0
        detail = label + signature
        members.append({"label": label, "detail": detail, "arg_count": arg_count})
    result[name] = {"aliases": config.get("aliases", [name]), "factories": config.get("factories", []), "members": members}
print(json.dumps(result))
`;
  const output = execFileSync(commands.python, ["-c", script, JSON.stringify(languageProfile)], { encoding: "utf8" });
  const rawTypes = JSON.parse(output);
  const types = {};
  for (const [name, config] of Object.entries(rawTypes)) {
    types[name] = {
      aliases: config.aliases,
      factories: config.factories,
      members: dedupeMembers(config.members.map((item) => member(name, item.label, item.detail, item.arg_count)))
    };
  }
  return { types };
}

function generateTypeScript(languageProfile) {
  const wanted = new Map(Object.entries(languageProfile.types).map(([name, config]) => [config.probe, { name, config }]));
  const collected = new Map([...wanted.values()].map(({ name }) => [name, []]));
  const libDir = dirname(fileURLToPath(import.meta.resolve("typescript")));
  for (const filename of readdirSync(libDir).filter((name) => /^lib\..*\.d\.ts$/.test(name))) {
    const source = ts.createSourceFile(filename, readFileSync(join(libDir, filename), "utf8"), ts.ScriptTarget.Latest, true);
    walkTS(source, (node) => {
      if (!ts.isInterfaceDeclaration(node)) return;
      const target = wanted.get(node.name.text);
      if (!target) return;
      for (const item of node.members) {
        if (!ts.isMethodSignature(item) || !item.name) continue;
        const label = item.name.getText(source).replace(/^['"]|['"]$/g, "");
        if (!/^[A-Za-z_$][\w$]*$/.test(label)) continue;
        collected.get(target.name).push(member(target.name, label, item.getText(source), item.parameters.length));
      }
    });
  }
  const types = {};
  for (const { name, config } of wanted.values()) {
    types[name] = typeEntry(config, collected.get(name));
  }
  return { types };
}

function generateCpp(languageProfile) {
  const types = {};
  for (const [name, config] of Object.entries(languageProfile.types)) {
    const line = `void complete() { ${config.probe} value; value.`;
    const source = `#include <${config.include}>\n${line}\n}\n`;
    const result = spawnSync(commands.clang, [
      "-x", "c++", "-std=c++20", "-fsyntax-only", "-Xclang",
      `-code-completion-at=-:2:${line.length + 1}`, "-"
    ], { input: source, encoding: "utf8", maxBuffer: 20 * 1024 * 1024 });
    if (result.status !== 0 && !result.stdout.includes("COMPLETION:")) {
      throw new Error(`clang completion failed for ${config.probe}: ${result.stderr}`);
    }
    const members = [];
    for (const line of `${result.stdout}\n${result.stderr}`.split("\n")) {
      const match = line.match(/^COMPLETION:\s+([^ ]+)(?:\s+\(Inaccessible\))?\s+:\s+(.+)$/);
      if (!match || line.includes("(Inaccessible)") || /^[_~]/.test(match[1]) || match[1].startsWith("operator")) continue;
      const argCount = [...match[2].matchAll(/<#[^#]+#>/g)].length;
      members.push(member(name, match[1], cleanClangDetail(match[2]), argCount));
    }
    types[name] = typeEntry(config, members);
  }
  return { types };
}

function inheritLanguage(base, config) {
  return {
    types: {
      ...structuredClone(base.types),
      ...(config.types ?? {})
    }
  };
}

function typeEntry(config, members) {
  return {
    aliases: config.aliases ?? [],
    factories: config.factories ?? [],
    members: dedupeMembers(members)
  };
}

function member(typeName, label, detail, argCount) {
  const placeholders = Array.from({ length: Math.min(argCount, 6) }, (_, index) => `$${index + 1}`);
  return {
    id: `${typeName}.${label}`,
    label,
    detail,
    insert_text: `${label}(${placeholders.join(", ")}${placeholders.length ? "" : "$0"})`,
    kind: "method",
    source: "oj_sdk_index",
    rank: 5
  };
}

function dedupeMembers(members) {
  const best = new Map();
  for (const item of members) {
    const current = best.get(item.label);
    if (!current || item.insert_text.length < current.insert_text.length) best.set(item.label, item);
  }
  return [...best.values()].sort((a, b) => a.label.localeCompare(b.label));
}

function splitTopLevel(value) {
  if (!value.trim()) return [];
  const out = [];
  let depth = 0;
  let start = 0;
  for (let index = 0; index < value.length; index++) {
    const char = value[index];
    if ("<([".includes(char)) depth++;
    if (">)]".includes(char)) depth--;
    if (char === "," && depth === 0) {
      out.push(value.slice(start, index));
      start = index + 1;
    }
  }
  out.push(value.slice(start));
  return out.filter((item) => item.trim());
}

function matchingParen(value, openIndex) {
  let depth = 0;
  for (let index = openIndex; index < value.length; index++) {
    if (value[index] === "(") depth++;
    if (value[index] === ")") {
      depth--;
      if (depth === 0) return index;
    }
  }
  return -1;
}

function cleanJavaDetail(value) {
  return value.replace(/^public\s+/, "").replace(/\b(?:abstract|default|final|native|static|synchronized)\s+/g, "");
}

function cleanClangDetail(value) {
  return value.replace(/\[#(.*?)#\]/g, "$1 ").replace(/<#(.*?)#>/g, "$1");
}

function commandVersion(command, args) {
  return execFileSync(command, args, { encoding: "utf8" }).trim().split("\n")[0];
}

function firstAvailableCommand(candidates) {
  for (const command of candidates) {
    if (spawnSync(command, ["--version"], { encoding: "utf8" }).status === 0) return command;
  }
  throw new Error(`none of the required commands are available: ${candidates.join(", ")}`);
}

function walkTS(node, visit) {
  visit(node);
  node.forEachChild((child) => walkTS(child, visit));
}
