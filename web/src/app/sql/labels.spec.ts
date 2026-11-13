import { ResultTab } from '@exa/store/sql/sql.state';
import { lineAndColumn, resultSummary, statementLabel, tabLabel } from './labels';

const tab = (overrides: Partial<ResultTab>): ResultTab => ({
  sql: 'SELECT 1',
  from: 0,
  to: 8,
  status: 'succeeded',
  ...overrides,
});

describe('SQL labels', () => {
  it('names statements by their keyword and object kind', () => {
    expect(statementLabel('-- revenue\nselect 1')).toBe('SELECT');
    expect(statementLabel('OPEN SCHEMA RETAIL')).toBe('OPEN SCHEMA');
    expect(statementLabel('create or replace table T (A INT)')).toBe('CREATE TABLE');
    expect(statementLabel('DROP SCHEMA IF EXISTS S CASCADE')).toBe('DROP SCHEMA');
  });

  it('labels tabs by how their statement ended', () => {
    const resultSet = {
      type: 'result_set' as const,
      columns: [],
      rows: [],
      total_rows: 2_964_624,
      truncated: true,
      duration_ms: 410,
      schema: null,
    };
    expect(tabLabel(tab({ outcome: resultSet }), 0)).toBe('SELECT · 2,964,624 rows');
    expect(
      tabLabel(
        tab({ sql: 'OPEN SCHEMA R', outcome: { type: 'row_count', row_count: 0, duration_ms: 1, schema: 'R' } }),
        0
      )
    ).toBe('OPEN SCHEMA');
    expect(
      tabLabel(
        tab({ sql: 'DELETE FROM T', outcome: { type: 'row_count', row_count: 1, duration_ms: 1, schema: null } }),
        0
      )
    ).toBe('DELETE · 1 row');
    expect(tabLabel(tab({ sql: 'CREATE TABLE T AS SELECT 1', status: 'running', startedAt: 1_000 }), 15_500)).toBe(
      'CREATE TABLE · 14 s'
    );
    expect(tabLabel(tab({ status: 'waiting' }), 0)).toBe('waiting');
    expect(tabLabel(tab({ status: 'failed' }), 0)).toBe('SELECT · failed');
    expect(tabLabel(tab({ status: 'cancelled' }), 0)).toBe('SELECT · cancelled');
    expect(tabLabel(tab({ status: 'not_run' }), 0)).toBe('not run');
  });

  it('summarizes results with their row count and duration', () => {
    const rows = Array.from({ length: 10_000 }, () => ['x']);
    expect(
      resultSummary(
        tab({
          outcome: {
            type: 'result_set',
            columns: [],
            rows,
            total_rows: 2_964_624,
            truncated: true,
            duration_ms: 410,
            schema: null,
          },
        })
      )
    ).toBe('First 10,000 of 2,964,624 rows · 0.41 s');
    expect(
      resultSummary(
        tab({
          outcome: {
            type: 'result_set',
            columns: [],
            rows: rows.slice(0, 5),
            total_rows: 5,
            truncated: false,
            duration_ms: 80,
            schema: null,
          },
        })
      )
    ).toBe('5 rows · 0.08 s');
    expect(resultSummary(tab({ outcome: { type: 'row_count', row_count: 12, duration_ms: 10, schema: null } }))).toBe(
      '12 rows affected · 0.01 s'
    );
  });

  it('finds the line and column of an offset', () => {
    const text = 'OPEN SCHEMA R;\n\nSELECT r.REGOIN_NAME';
    expect(lineAndColumn(text, text.indexOf('REGOIN'))).toEqual({ line: 3, column: 10 });
    expect(lineAndColumn(text, 0)).toEqual({ line: 1, column: 1 });
  });
});
