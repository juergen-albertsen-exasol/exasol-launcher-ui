import type { ResultTab } from '@exa/store/sql/sql.state';

const OBJECT_VERBS = new Set(['CREATE', 'DROP', 'ALTER', 'OPEN', 'CLOSE', 'TRUNCATE', 'RENAME', 'COMMENT']);
const MODIFIERS = new Set(['OR', 'REPLACE', 'FORCE', 'IF', 'NOT', 'EXISTS', 'GLOBAL', 'LOCAL']);
const ROW_VERBS = new Set(['INSERT', 'UPDATE', 'DELETE', 'MERGE', 'IMPORT', 'EXPORT']);

const words = (sql: string): string[] =>
  sql
    .replace(/^(\s|--[^\n]*(\n|$)|\/\*[\s\S]*?\*\/)*/, '')
    .split(/[^A-Za-z_]+/)
    .filter(Boolean)
    .map(word => word.toUpperCase());

/** Names a statement by its first keyword, and the kind of object for statements such as CREATE TABLE. */
export function statementLabel(sql: string): string {
  const [first = '', ...rest] = words(sql);
  if (!OBJECT_VERBS.has(first)) return first;
  const object = rest.find(word => !MODIFIERS.has(word));
  return object ? `${first} ${object}` : first;
}

export const formatCount = (count: number): string => count.toLocaleString('en-US');

export const formatRows = (count: number): string => `${formatCount(count)} ${count === 1 ? 'row' : 'rows'}`;

export const formatDuration = (ms: number): string => `${(ms / 1000).toFixed(2)} s`;

export const formatElapsed = (ms: number): string => `${Math.floor(ms / 1000)} s`;

/** The text of a result tab: what ran and how it ended. */
export function tabLabel(tab: ResultTab, now: number): string {
  const label = statementLabel(tab.sql);
  switch (tab.status) {
    case 'waiting':
      return 'waiting';
    case 'not_run':
      return 'not run';
    case 'running':
      return `${label} · ${formatElapsed(now - (tab.startedAt ?? now))}`;
    case 'failed':
      return `${label} · failed`;
    case 'cancelled':
      return `${label} · cancelled`;
    case 'succeeded': {
      const outcome = tab.outcome;
      if (outcome?.type === 'result_set') return `${label} · ${formatRows(outcome.total_rows)}`;
      if (outcome?.type === 'row_count' && ROW_VERBS.has(words(tab.sql)[0] ?? '')) {
        return `${label} · ${formatRows(outcome.row_count)}`;
      }
      return label;
    }
  }
}

/** The line below a result: how many rows, and how long the statement took. */
export function resultSummary(tab: ResultTab): string {
  const outcome = tab.outcome;
  if (!outcome) return '';
  const duration = formatDuration(outcome.duration_ms);
  if (outcome.type === 'row_count') return `${formatRows(outcome.row_count)} affected · ${duration}`;
  if (outcome.truncated) {
    return `First ${formatCount(outcome.rows.length)} of ${formatRows(outcome.total_rows)} · ${duration}`;
  }
  return `${formatRows(outcome.total_rows)} · ${duration}`;
}

/** The editor's line and column, both from 1, of an offset in text. */
export function lineAndColumn(text: string, offset: number): { line: number; column: number } {
  const before = text.slice(0, offset);
  const line = before.split('\n').length;
  return { line, column: offset - (before.lastIndexOf('\n') + 1) + 1 };
}
