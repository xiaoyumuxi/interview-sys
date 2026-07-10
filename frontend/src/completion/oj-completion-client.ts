import type { OJCompletionRequest, OJCompletionResponse, OJCompletionSuggestion } from "./types";

let worker: Worker | null = null;
let requestSequence = 0;
const pending = new Map<number, {
  resolve: (value: OJCompletionSuggestion[]) => void;
  timeout: number;
}>();

export function suggestOJCompletions(input: Omit<OJCompletionRequest, "requestId">): Promise<OJCompletionSuggestion[]> {
  const activeWorker = ensureWorker();
  const requestId = ++requestSequence;
  return new Promise((resolve) => {
    const timeout = window.setTimeout(() => {
      pending.delete(requestId);
      resolve([]);
    }, 1800);
    pending.set(requestId, { resolve, timeout });
    activeWorker.postMessage({ ...input, requestId } satisfies OJCompletionRequest);
  });
}

export function disposeOJCompletionWorker(): void {
  worker?.terminate();
  worker = null;
  for (const request of pending.values()) {
    window.clearTimeout(request.timeout);
    request.resolve([]);
  }
  pending.clear();
}

function ensureWorker(): Worker {
  if (worker) return worker;
  worker = new Worker(new URL("./oj-completion.worker.ts", import.meta.url), { type: "module" });
  worker.addEventListener("message", (event: MessageEvent<OJCompletionResponse>) => {
    const request = pending.get(event.data.requestId);
    if (!request) return;
    window.clearTimeout(request.timeout);
    pending.delete(event.data.requestId);
    request.resolve(event.data.error ? [] : event.data.suggestions);
  });
  worker.addEventListener("error", () => disposeOJCompletionWorker());
  return worker;
}
