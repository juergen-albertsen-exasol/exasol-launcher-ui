import type { ResultTab } from '@exa/store/sql/sql.state';
import type { HistoryEntry } from './storage';

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

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;

const time = (date: Date) =>
  `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;

/** When a statement ran: minutes ago within the hour, else the day and time. */
export function formatWhen(at: string, now: number): string {
  const date = new Date(at);
  const ago = now - date.getTime();
  if (ago < MINUTE_MS) return 'just now';
  if (ago < HOUR_MS) return `${Math.floor(ago / MINUTE_MS)} min ago`;
  const today = new Date(now);
  today.setHours(0, 0, 0, 0);
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);
  if (date >= today) return `Today ${time(date)}`;
  if (date >= yesterday) return `Yesterday ${time(date)}`;
  const day = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(
    2,
    '0'
  )}`;
  return `${day} ${time(date)}`;
}

/** How a statement in the history ended: its rows and duration, or its error. */
export function historySummary(entry: HistoryEntry): string {
  const duration = formatDuration(entry.ms);
  if (entry.outcome === 'failed') return entry.error ?? 'failed';
  if (entry.outcome === 'cancelled') return `cancelled · ${duration}`;
  if (entry.rows !== undefined) return `${formatRows(entry.rows)} · ${duration}`;
  if (entry.affected !== undefined && ROW_VERBS.has(words(entry.sql)[0] ?? '')) {
    return `${formatRows(entry.affected)} affected · ${duration}`;
  }
  return duration;
}
