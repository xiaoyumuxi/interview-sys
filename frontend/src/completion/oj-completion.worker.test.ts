import { fileURLToPath } from "node:url";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { Language, Parser } from "web-tree-sitter";
import completionIndexJSON from "../generated/oj-completion-index.json";
import { suggestionsAtCursor } from "./oj-completion.worker";
import type { OJCompletionIndex } from "./types";

const completionIndex = completionIndexJSON as OJCompletionIndex;
const parsers = new Map<string, Parser>();

beforeAll(async () => {
  await Parser.init();
});

afterAll(() => {
  for (const parser of parsers.values()) parser.delete();
});

describe("Tree-sitter OJ completions", () => {
  const cases = [
    { language: "java", source: "Map<String, Integer> counts = new HashMap<>();\ncounts.pu", labels: ["put"] },
    { language: "go", source: "var builder strings.Builder\nbuilder.Write", labels: ["WriteString"] },
    { language: "python", source: "from collections import deque as D\nqueue = D()\nqueue.pop", labels: ["pop", "popleft"] },
    { language: "javascript", source: "const seen = new Map();\nseen.ha", labels: ["has"] },
    { language: "typescript", source: "const values: number[] = [];\nvalues.fi", labels: ["filter"] },
    { language: "cpp", source: "std::vector<int> values;\nvalues.push", labels: ["push_back"] }
  ];

  for (const testCase of cases) {
    it(`uses the ${testCase.language} grammar and generated SDK index`, async () => {
      const labels = await complete(testCase.language, testCase.source);
      expect(labels).toEqual(expect.arrayContaining(testCase.labels));
    });
  }

  it("does not leak a shadowed binding from another JavaScript function", async () => {
    const source = [
      "const value = new Map();",
      "function other() {",
      "  const value = [];",
      "  value.push(1);",
      "}",
      "value.se"
    ].join("\n");
    const labels = await complete("javascript", source);
    expect(labels).toContain("set");
    expect(labels).not.toContain("search");
  });
});

async function complete(language: string, source: string): Promise<string[]> {
  const parser = await parserFor(language);
  const tree = parser.parse(source);
  if (!tree) throw new Error(`failed to parse ${language}`);
  const languageIndex = completionIndex.languages[language];
  if (!languageIndex) throw new Error(`missing completion index for ${language}`);
  try {
    const result = suggestionsAtCursor(
      language,
      languageIndex,
      tree.rootNode,
      source,
      source.length,
      source.match(/[A-Za-z_$][\w$]*$/)?.[0] ?? ""
    );
    return result.items.map((item) => item.label);
  } finally {
    tree.delete();
  }
}

async function parserFor(language: string): Promise<Parser> {
  const existing = parsers.get(language);
  if (existing) return existing;
  const parser = new Parser();
  const grammarPath = fileURLToPath(new URL(
    `../../node_modules/@repomix/tree-sitter-wasms/out/tree-sitter-${language}.wasm`,
    import.meta.url
  ));
  parser.setLanguage(await Language.load(grammarPath));
  parsers.set(language, parser);
  return parser;
}
