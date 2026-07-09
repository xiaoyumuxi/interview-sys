import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const index = JSON.parse(readFileSync(join(root, "src", "generated", "oj-completion-index.json"), "utf8"));

assert.equal(index.schema_version, "oj.completion.index.v1");
assertMember("java", "Map", "put");
assertMember("go", "strings.Builder", "WriteString");
assertMember("python", "deque", "popleft");
assertMember("javascript", "Map", "has");
assertMember("typescript", "Array", "filter");
assertMember("cpp", "vector", "push_back");
assertMember("cpp", "unordered_map", "reserve");

for (const [language, languageIndex] of Object.entries(index.languages)) {
  assert.ok(Object.keys(languageIndex.types).length > 0, `${language} index is empty`);
  for (const [typeName, type] of Object.entries(languageIndex.types)) {
    assert.ok(type.aliases.length > 0, `${language}.${typeName} has no aliases`);
    assert.ok(type.members.length > 0, `${language}.${typeName} has no generated members`);
    assert.equal(new Set(type.members.map((item) => item.label)).size, type.members.length, `${language}.${typeName} has duplicate members`);
  }
}

process.stdout.write("OJ completion index smoke passed\n");

function assertMember(language, typeName, label) {
  const type = index.languages[language]?.types[typeName];
  assert.ok(type, `missing ${language}.${typeName}`);
  assert.ok(type.members.some((item) => item.label === label), `missing ${language}.${typeName}.${label}`);
}
