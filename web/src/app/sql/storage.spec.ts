import {
  HISTORY_LIMIT,
  HistoryEntry,
  addHistory,
  appendText,
  clearHistory,
  loadHistory,
  loadText,
  saveText,
} from './storage';

const entry = (sql: string): HistoryEntry => ({ sql, at: '2026-11-13T10:00:00Z', ms: 5, outcome: 'succeeded' });

describe('SQL storage', () => {
  beforeEach(() => localStorage.clear());

  it('keeps the text per deployment', () => {
    saveText('default', 'SELECT 1');
    saveText('test', 'SELECT 2');

    expect(loadText('default')).toBe('SELECT 1');
    expect(loadText('test')).toBe('SELECT 2');
    expect(loadText('other')).toBeUndefined();
    expect(localStorage.getItem('exasol-launcher-ui:sql:default:text')).toBe('SELECT 1');
  });

  it('adds a statement on a new line after the text', () => {
    saveText('default', 'SELECT 1;\n\n');

    appendText('default', 'SELECT 2;');
    appendText('test', 'SELECT 3;');

    expect(loadText('default')).toBe('SELECT 1;\nSELECT 2;');
    expect(loadText('test')).toBe('SELECT 3;');
  });

  it('keeps the newest statements first, up to the limit', () => {
    for (let index = 0; index < HISTORY_LIMIT + 5; index++) addHistory('default', entry(`SELECT ${index}`));

    const history = loadHistory('default');
    expect(history).toHaveLength(HISTORY_LIMIT);
    expect(history[0].sql).toBe(`SELECT ${HISTORY_LIMIT + 4}`);
    expect(loadHistory('test')).toEqual([]);
  });

  it('clears the history of one deployment', () => {
    addHistory('default', entry('SELECT 1'));
    addHistory('test', entry('SELECT 2'));

    clearHistory('default');

    expect(loadHistory('default')).toEqual([]);
    expect(loadHistory('test')).toHaveLength(1);
  });

  it('ignores a corrupt history', () => {
    localStorage.setItem('exasol-launcher-ui:sql:default:history', '{');

    expect(loadHistory('default')).toEqual([]);
  });
});
