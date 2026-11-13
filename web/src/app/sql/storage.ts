/**
 * The editor's text and history per deployment, kept in this browser's localStorage. Results and the
 * password are never stored. Without storage, the editor works and keeps nothing.
 */

export type HistoryOutcome = 'succeeded' | 'failed' | 'cancelled';

export interface HistoryEntry {
  sql: string;
  /** When it ran, as an ISO timestamp. */
  at: string;
  ms: number;
  outcome: HistoryOutcome;
  /** The total rows of a result set. */
  rows?: number;
  /** The rows a statement without a result set affected. */
  affected?: number;
  /** The database's error code and message. */
  error?: string;
}

export const HISTORY_LIMIT = 100;

const textKey = (deployment: string) => `exasol-local-ui:sql:${deployment}:text`;
const historyKey = (deployment: string) => `exasol-local-ui:sql:${deployment}:history`;

function storage(): Storage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

export function loadText(deployment: string): string | undefined {
  try {
    return storage()?.getItem(textKey(deployment)) ?? undefined;
  } catch {
    return undefined;
  }
}

export function saveText(deployment: string, text: string): void {
  try {
    storage()?.setItem(textKey(deployment), text);
  } catch {
    // Full or unavailable storage only loses the convenience.
  }
}

/** The statements run, newest first. */
export function loadHistory(deployment: string): HistoryEntry[] {
  try {
    const parsed: unknown = JSON.parse(storage()?.getItem(historyKey(deployment)) ?? '[]');
    return Array.isArray(parsed) ? (parsed as HistoryEntry[]) : [];
  } catch {
    return [];
  }
}

export function addHistory(deployment: string, entry: HistoryEntry): HistoryEntry[] {
  const history = [entry, ...loadHistory(deployment)].slice(0, HISTORY_LIMIT);
  try {
    storage()?.setItem(historyKey(deployment), JSON.stringify(history));
  } catch {
    // As above.
  }
  return history;
}

export function clearHistory(deployment: string): void {
  try {
    storage()?.removeItem(historyKey(deployment));
  } catch {
    // As above.
  }
}
