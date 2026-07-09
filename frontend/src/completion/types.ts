export interface OJCompletionSuggestion {
  id: string;
  label: string;
  detail: string;
  insert_text: string;
  kind: string;
  source: "oj_sdk_index";
  rank: number;
}

export interface OJCompletionRequest {
  requestId: number;
  language: string;
  source: string;
  cursorOffset: number;
  prefix: string;
}

export interface OJCompletionResponse {
  requestId: number;
  suggestions: OJCompletionSuggestion[];
  receiverType?: string;
  parser: "tree-sitter";
  error?: string;
}

export interface OJCompletionIndexType {
  aliases: string[];
  factories: string[];
  members: OJCompletionSuggestion[];
}

export interface OJCompletionLanguageIndex {
  types: Record<string, OJCompletionIndexType>;
}

export interface OJCompletionIndex {
  schema_version: string;
  languages: Record<string, OJCompletionLanguageIndex>;
}
