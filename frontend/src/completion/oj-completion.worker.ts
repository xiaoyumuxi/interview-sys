/// <reference lib="webworker" />

import { Language, Parser, type Node as SyntaxNode, type Tree } from "web-tree-sitter";
import parserWasmURL from "web-tree-sitter/tree-sitter.wasm?url";
import cppWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-cpp.wasm?url";
import goWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-go.wasm?url";
import javaWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-java.wasm?url";
import javascriptWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-javascript.wasm?url";
import pythonWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-python.wasm?url";
import typescriptWasmURL from "@repomix/tree-sitter-wasms/out/tree-sitter-typescript.wasm?url";
import completionIndexJSON from "../generated/oj-completion-index.json";
import type {
  OJCompletionIndex,
  OJCompletionLanguageIndex,
  OJCompletionRequest,
  OJCompletionResponse,
  OJCompletionSuggestion
} from "./types";

const completionIndex = completionIndexJSON as OJCompletionIndex;
const grammarURLs: Record<string, string> = {
  cpp: cppWasmURL,
  go: goWasmURL,
  java: javaWasmURL,
  javascript: javascriptWasmURL,
  python: pythonWasmURL,
  typescript: typescriptWasmURL
};
const isCompletionWorker = typeof WorkerGlobalScope !== "undefined" && self instanceof WorkerGlobalScope;
const parserReady = isCompletionWorker
  ? Parser.init({ locateFile: () => parserWasmURL })
  : Promise.resolve();
const parsers = new Map<string, Promise<Parser>>();
const documents = new Map<string, { source: string; tree: Tree }>();
const encoder = new TextEncoder();

if (isCompletionWorker) {
  self.addEventListener("message", (event: MessageEvent<OJCompletionRequest>) => {
    void complete(event.data).then((result) => self.postMessage(result));
  });
}

async function complete(request: OJCompletionRequest): Promise<OJCompletionResponse> {
  try {
    const language = normalizeLanguage(request.language);
    const languageIndex = completionIndex.languages[language];
    if (!languageIndex || !grammarURLs[language]) {
      return response(request.requestId, []);
    }
    const parser = await parserFor(language);
    const tree = updateTree(language, parser, request.source);
    const suggestions = suggestionsAtCursor(
      language,
      languageIndex,
      tree.rootNode,
      request.source,
      request.cursorOffset,
      request.prefix
    );
    return response(request.requestId, suggestions.items, suggestions.receiverType);
  } catch (error) {
    return { ...response(request.requestId, []), error: error instanceof Error ? error.message : String(error) };
  }
}

function response(requestId: number, suggestions: OJCompletionSuggestion[], receiverType?: string): OJCompletionResponse {
  return { requestId, suggestions, receiverType, parser: "tree-sitter" };
}

async function parserFor(language: string): Promise<Parser> {
  const existing = parsers.get(language);
  if (existing) return existing;
  const loading = (async () => {
    await parserReady;
    const grammarURL = grammarURLs[language];
    if (!grammarURL) throw new Error(`missing Tree-sitter grammar for ${language}`);
    const parser = new Parser();
    parser.setLanguage(await Language.load(grammarURL));
    return parser;
  })();
  parsers.set(language, loading);
  return loading;
}

function updateTree(language: string, parser: Parser, source: string): Tree {
  const previous = documents.get(language);
  if (!previous) {
    const tree = parser.parse(source);
    if (!tree) throw new Error(`Tree-sitter could not parse ${language}`);
    documents.set(language, { source, tree });
    return tree;
  }
  if (previous.source === source) return previous.tree;

  const edit = sourceEdit(previous.source, source);
  previous.tree.edit(edit);
  const tree = parser.parse(source, previous.tree);
  if (!tree) throw new Error(`Tree-sitter could not update ${language}`);
  previous.tree.delete();
  documents.set(language, { source, tree });
  return tree;
}

function sourceEdit(oldSource: string, newSource: string) {
  let start = 0;
  const sharedLength = Math.min(oldSource.length, newSource.length);
  while (start < sharedLength && oldSource[start] === newSource[start]) start++;
  start = safeUTF16Boundary(oldSource, start);
  let oldEnd = oldSource.length;
  let newEnd = newSource.length;
  while (oldEnd > start && newEnd > start && oldSource[oldEnd - 1] === newSource[newEnd - 1]) {
    oldEnd--;
    newEnd--;
  }
  oldEnd = safeUTF16Boundary(oldSource, oldEnd);
  newEnd = safeUTF16Boundary(newSource, newEnd);
  return {
    startIndex: byteOffset(oldSource, start),
    oldEndIndex: byteOffset(oldSource, oldEnd),
    newEndIndex: byteOffset(newSource, newEnd),
    startPosition: pointAt(oldSource, start),
    oldEndPosition: pointAt(oldSource, oldEnd),
    newEndPosition: pointAt(newSource, newEnd)
  };
}

function safeUTF16Boundary(value: string, offset: number): number {
  if (offset <= 0 || offset >= value.length) return offset;
  const previous = value.charCodeAt(offset - 1);
  const current = value.charCodeAt(offset);
  return previous >= 0xd800 && previous <= 0xdbff && current >= 0xdc00 && current <= 0xdfff
    ? offset - 1
    : offset;
}

export function suggestionsAtCursor(
  language: string,
  languageIndex: OJCompletionLanguageIndex,
  root: SyntaxNode,
  source: string,
  cursorOffset: number,
  requestedPrefix: string
): { items: OJCompletionSuggestion[]; receiverType?: string } {
  const context = memberContext(source.slice(0, cursorOffset));
  if (!context) return { items: [] };
  const aliases = typeAliases(languageIndex);
  const factories = typeFactories(languageIndex, aliases);
  const importAliases = new Map<string, string>();
  const symbols = new Map<string, string>();
  const cursorByte = byteOffset(source, cursorOffset);
  const visibleScopes = visibleScopeIDs(root, cursorByte);

  walk(root, cursorByte, (node) => {
    if (!nodeIsVisible(node, root, visibleScopes)) return;
    collectImportAlias(node, aliases, importAliases);
    collectDeclaration(language, node, aliases, factories, importAliases, symbols);
  });

  const receiverType = symbols.get(context.receiver)
    ?? importAliases.get(context.receiver)
    ?? aliases.get(normalizeTypeText(language, context.receiver));
  if (!receiverType) return { items: [] };
  const type = languageIndex.types[receiverType];
  if (!type) return { items: [] };
  const prefix = (context.prefix || requestedPrefix).toLowerCase();
  const items = type.members
    .filter((item) => !prefix || item.label.toLowerCase().startsWith(prefix) || item.label.toLowerCase().includes(prefix))
    .sort((a, b) => completionScore(a.label, prefix) - completionScore(b.label, prefix) || a.label.localeCompare(b.label))
    .slice(0, 40);
  return { items, receiverType };
}

const lexicalScopeTypes = new Set([
  "block",
  "class_body",
  "compound_statement",
  "constructor_declaration",
  "function_declaration",
  "function_definition",
  "lambda_expression",
  "method_declaration",
  "module",
  "program",
  "source_file",
  "statement_block",
  "translation_unit"
]);

function visibleScopeIDs(root: SyntaxNode, cursorByte: number): Set<number> {
  const scopes = new Set<number>([root.id]);
  let current: SyntaxNode | null = root.descendantForIndex(Math.max(0, cursorByte - 1));
  while (current) {
    if (lexicalScopeTypes.has(current.type)) scopes.add(current.id);
    current = current.parent;
  }
  return scopes;
}

function nodeIsVisible(node: SyntaxNode, root: SyntaxNode, visibleScopes: Set<number>): boolean {
  let current = node.parent;
  while (current) {
    if (lexicalScopeTypes.has(current.type)) return visibleScopes.has(current.id);
    current = current.parent;
  }
  return visibleScopes.has(root.id);
}

function collectDeclaration(
  language: string,
  node: SyntaxNode,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (language === "java") collectJavaDeclaration(node, aliases, factories, imports, symbols);
  if (language === "go") collectGoDeclaration(node, aliases, factories, imports, symbols);
  if (language === "python") collectPythonDeclaration(node, aliases, factories, imports, symbols);
  if (language === "javascript" || language === "typescript") collectECMADeclaration(node, language, aliases, factories, imports, symbols);
  if (language === "cpp") collectCppDeclaration(node, aliases, symbols);
}

function collectJavaDeclaration(
  node: SyntaxNode,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (["local_variable_declaration", "field_declaration"].includes(node.type)) {
    const declared = node.childForFieldName("type")?.text ?? "";
    for (const child of compactNodes(node.namedChildren)) {
      if (child.type !== "variable_declarator") continue;
      const name = child.childForFieldName("name")?.text;
      const value = child.childForFieldName("value");
      bind(name, declared === "var" ? inferExpressionType("java", value, aliases, factories, imports) : declared, aliases, symbols);
    }
  }
  if (node.type === "formal_parameter") {
    bind(node.childForFieldName("name")?.text, node.childForFieldName("type")?.text, aliases, symbols);
  }
}

function collectGoDeclaration(
  node: SyntaxNode,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (node.type === "var_spec" || node.type === "parameter_declaration") {
    const names = compactNodes(node.namedChildren).filter((child) => child.type === "identifier");
    const type = node.childForFieldName("type")?.text;
    for (const name of names) bind(name.text, type, aliases, symbols);
  }
  if (node.type === "short_var_declaration") {
    const left = node.childForFieldName("left");
    const right = node.childForFieldName("right");
    const names = compactNodes(left?.namedChildren ?? []).filter((child) => child.type === "identifier");
    const values = compactNodes(right?.namedChildren ?? []);
    names.forEach((name, index) => bind(
      name.text,
      inferExpressionType("go", values[index], aliases, factories, imports),
      aliases,
      symbols
    ));
  }
}

function collectPythonDeclaration(
  node: SyntaxNode,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (node.type === "assignment") {
    const left = node.childForFieldName("left");
    const right = node.childForFieldName("right");
    if (left?.type === "identifier") {
      bind(left.text, inferExpressionType("python", right, aliases, factories, imports), aliases, symbols);
    }
  }
  if (node.type === "typed_parameter") {
    const name = compactNodes(node.namedChildren).find((child) => child.type === "identifier")?.text;
    bind(name, node.childForFieldName("type")?.text, aliases, symbols);
  }
}

function collectECMADeclaration(
  node: SyntaxNode,
  language: string,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (node.type === "variable_declarator") {
    const name = node.childForFieldName("name")?.text;
    const annotated = node.childForFieldName("type")?.text.replace(/^:\s*/, "");
    const inferred = inferExpressionType(language, node.childForFieldName("value"), aliases, factories, imports);
    bind(name, annotated || inferred, aliases, symbols);
  }
  if (["required_parameter", "optional_parameter"].includes(node.type)) {
    const name = node.childForFieldName("pattern")?.text ?? node.childForFieldName("name")?.text;
    bind(name, node.childForFieldName("type")?.text.replace(/^:\s*/, ""), aliases, symbols);
  }
}

function collectCppDeclaration(node: SyntaxNode, aliases: Map<string, string>, symbols: Map<string, string>): void {
  if (node.type !== "declaration" && node.type !== "parameter_declaration") return;
  const type = node.childForFieldName("type")?.text;
  const declarator = node.childForFieldName("declarator");
  bind(identifierWithin(declarator), type, aliases, symbols);
}

function collectImportAlias(node: SyntaxNode, aliases: Map<string, string>, imports: Map<string, string>): void {
  if (node.type !== "aliased_import") return;
  const imported = node.childForFieldName("name")?.text;
  const alias = node.childForFieldName("alias")?.text;
  const canonical = imported ? aliases.get(normalizeTypeText("python", imported)) : undefined;
  if (alias && canonical) imports.set(alias, canonical);
}

function inferExpressionType(
  language: string,
  node: SyntaxNode | null | undefined,
  aliases: Map<string, string>,
  factories: Map<string, string>,
  imports: Map<string, string>
): string | undefined {
  if (!node) return undefined;
  if (language === "java" && node.type === "object_creation_expression") {
    return canonicalFromText(node.childForFieldName("type")?.text, aliases, imports);
  }
  if (language === "go" && node.type === "composite_literal") {
    return canonicalFromText(node.childForFieldName("type")?.text, aliases, imports);
  }
  if (["call", "call_expression"].includes(node.type)) {
    const fn = node.childForFieldName("function")?.text;
    return (fn && (factories.get(fn) ?? imports.get(fn) ?? canonicalFromText(fn, aliases, imports))) || undefined;
  }
  if (language === "javascript" || language === "typescript") {
    if (node.type === "new_expression") return canonicalFromText(node.childForFieldName("constructor")?.text, aliases, imports);
    if (node.type === "array") return aliases.get("Array");
    if (["string", "template_string"].includes(node.type)) return aliases.get("String");
  }
  if (language === "python") {
    if (["list", "list_comprehension"].includes(node.type)) return aliases.get("list");
    if (node.type === "dictionary") return aliases.get("dict");
    if (node.type === "set") return aliases.get("set");
    if (["string", "concatenated_string"].includes(node.type)) return aliases.get("str");
  }
  return undefined;
}

function bind(
  name: string | undefined,
  type: string | undefined,
  aliases: Map<string, string>,
  symbols: Map<string, string>
): void {
  if (!name || !type) return;
  const canonical = aliases.get(normalizeTypeText("", type)) ?? aliases.get(type.trim()) ?? type;
  if ([...aliases.values()].includes(canonical)) symbols.set(name, canonical);
}

function canonicalFromText(
  value: string | undefined,
  aliases: Map<string, string>,
  imports: Map<string, string>
): string | undefined {
  if (!value) return undefined;
  return imports.get(value) ?? aliases.get(normalizeTypeText("", value)) ?? aliases.get(value.trim());
}

function typeAliases(languageIndex: OJCompletionLanguageIndex): Map<string, string> {
  const aliases = new Map<string, string>();
  for (const [canonical, type] of Object.entries(languageIndex.types)) {
    aliases.set(normalizeTypeText("", canonical), canonical);
    for (const alias of type.aliases) aliases.set(normalizeTypeText("", alias), canonical);
  }
  if (languageIndex.types.Array) aliases.set("array", "Array");
  return aliases;
}

function typeFactories(languageIndex: OJCompletionLanguageIndex, aliases: Map<string, string>): Map<string, string> {
  const factories = new Map<string, string>();
  for (const [canonical, type] of Object.entries(languageIndex.types)) {
    for (const alias of type.aliases) factories.set(alias, canonical);
    for (const factory of type.factories) factories.set(factory, canonical);
  }
  for (const [alias, canonical] of aliases) factories.set(alias, canonical);
  return factories;
}

function normalizeTypeText(language: string, value: string): string {
  let type = value.trim().replace(/^:\s*/, "").replace(/\b(?:const|final)\b/g, "").trim();
  type = type.replace(/^[*&]+|[&*]+$/g, "").trim();
  if (/\[\]$/.test(type) && (language === "" || language === "javascript" || language === "typescript")) return "array";
  const generic = Math.min(...[type.indexOf("<"), type.indexOf("[")].filter((index) => index >= 0));
  if (Number.isFinite(generic)) type = type.slice(0, generic);
  return type.replace(/\s+/g, "");
}

function memberContext(value: string): { receiver: string; prefix: string } | null {
  const match = value.match(/([A-Za-z_$][\w$]*)\s*(?:\.|::)\s*([A-Za-z_$][\w$]*)?$/);
  const receiver = match?.[1];
  return receiver ? { receiver, prefix: match?.[2] ?? "" } : null;
}

function identifierWithin(node: SyntaxNode | null): string | undefined {
  if (!node) return undefined;
  if (node.type === "identifier") return node.text;
  const nested = node.childForFieldName("declarator");
  if (nested) return identifierWithin(nested);
  for (const child of compactNodes(node.namedChildren)) {
    const found = identifierWithin(child);
    if (found) return found;
  }
  return undefined;
}

function walk(node: SyntaxNode, cursorByte: number, visit: (node: SyntaxNode) => void): void {
  if (node.startIndex > cursorByte) return;
  visit(node);
  for (const child of compactNodes(node.namedChildren)) walk(child, cursorByte, visit);
}

function compactNodes(nodes: readonly (SyntaxNode | null)[]): SyntaxNode[] {
  return nodes.filter((node): node is SyntaxNode => node !== null);
}

function completionScore(label: string, prefix: string): number {
  if (!prefix) return 2;
  const normalized = label.toLowerCase();
  if (normalized === prefix) return 0;
  if (normalized.startsWith(prefix)) return 1;
  return 2;
}

function normalizeLanguage(language: string): string {
  const normalized = language.toLowerCase();
  if (normalized === "c++") return "cpp";
  if (normalized === "python3") return "python";
  return normalized;
}

function byteOffset(value: string, utf16Offset: number): number {
  return encoder.encode(value.slice(0, utf16Offset)).length;
}

function pointAt(value: string, utf16Offset: number): { row: number; column: number } {
  const before = value.slice(0, utf16Offset);
  const lines = before.split("\n");
  return { row: lines.length - 1, column: encoder.encode(lines.at(-1) ?? "").length };
}
