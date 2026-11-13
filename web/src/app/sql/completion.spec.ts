import { CompletionContext } from '@codemirror/autocomplete';
import { EditorState } from '@codemirror/state';
import type { Catalog } from './catalog';
import { namesCompletion } from './completion';

const catalog = (): Catalog => ({
  schemas: [
    {
      name: 'UPLOADS',
      virtual: false,
      columnsLoaded: true,
      tables: [
        {
          name: 'ORDERS',
          kind: 'table',
          columns: [
            { name: 'ORDER_ID', type: 'DECIMAL(18,0)' },
            { name: 'TOTAL_EUR', type: 'DECIMAL(12,2)' },
          ],
        },
        { name: 'CUSTOMERS', kind: 'table', columns: [{ name: 'CUSTOMER_ID', type: 'DECIMAL(18,0)' }] },
      ],
    },
    { name: 'S1', virtual: false, columnsLoaded: false, tables: [{ name: 'T2', kind: 'table', columns: [] }] },
  ],
});

function complete(doc: string, value = catalog(), onSchemasNeeded = jest.fn()) {
  const pos = doc.indexOf('|');
  const state = EditorState.create({ doc: doc.replace('|', '') });
  const result = namesCompletion(() => value, onSchemasNeeded)(new CompletionContext(state, pos, true));
  return { labels: result?.options.map(option => option.label) ?? [], from: result?.from, onSchemasNeeded };
}

describe('names completion', () => {
  it("offers a schema's tables and views by prefix", () => {
    const retail: Catalog = {
      schemas: [
        {
          name: 'RETAIL',
          virtual: false,
          columnsLoaded: false,
          tables: [
            { name: 'REGIONS', kind: 'table', columns: [] },
            { name: 'SALES', kind: 'table', columns: [] },
            { name: 'SALES_BY_DAY', kind: 'view', columns: [] },
          ],
        },
      ],
    };
    const pos = 'SELECT * FROM RETAIL.SA'.length;
    const state = EditorState.create({ doc: 'SELECT * FROM RETAIL.SA' });
    const result = namesCompletion(
      () => retail,
      () => undefined
    )(new CompletionContext(state, pos, false));
    const offered = result?.options.map(option => option.label).filter(label => /^SA/.test(label));
    expect(offered).toEqual(['SALES', 'SALES_BY_DAY']);
  });

  it('offers the columns of the tables the statement names', () => {
    const { labels } = complete('SELECT | FROM UPLOADS.ORDERS');
    expect(labels).toEqual(expect.arrayContaining(['ORDER_ID', 'TOTAL_EUR']));
    expect(labels).not.toContain('CUSTOMER_ID');
  });

  it("offers a schema's tables after the schema and a dot", () => {
    const { labels, from } = complete('SELECT * FROM S1.|');
    expect(labels).toEqual(['T2']);
    expect(from).toBe('SELECT * FROM S1.'.length);
  });

  it('offers schemas at the top level', () => {
    expect(complete('SELECT * FROM |').labels).toEqual(expect.arrayContaining(['UPLOADS', 'S1']));
  });

  it("offers a table's columns after schema, table, and a dot", () => {
    expect(complete('SELECT UPLOADS.ORDERS.| FROM UPLOADS.ORDERS').labels).toEqual(['ORDER_ID', 'TOTAL_EUR']);
  });

  it('asks for the columns of a named schema that are not read yet', () => {
    const { onSchemasNeeded } = complete('SELECT | FROM S1.T2');
    expect(onSchemasNeeded).toHaveBeenCalledWith(['S1']);
  });
});
