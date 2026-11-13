import { Catalog } from './catalog';
import { filterCatalog, sqlName } from './schema-tree';

const catalog: Catalog = {
  schemas: [
    {
      name: 'RETAIL',
      virtual: false,
      columnsLoaded: true,
      tables: [
        {
          name: 'CUSTOMERS',
          kind: 'table',
          columns: [{ name: 'CUSTOMER_ID', type: 'DECIMAL(18,0)' }],
        },
        {
          name: 'REGIONS',
          kind: 'table',
          columns: [
            { name: 'REGION_ID', type: 'DECIMAL(9,0)' },
            { name: 'REGION_NAME', type: 'VARCHAR(100) UTF8' },
          ],
        },
        {
          name: 'SALES',
          kind: 'table',
          columns: [
            { name: 'SALE_ID', type: 'DECIMAL(18,0)' },
            { name: 'REGION_ID', type: 'DECIMAL(9,0)' },
          ],
        },
      ],
    },
    { name: 'STAGING', virtual: false, columnsLoaded: true, tables: [] },
  ],
};

const names = (filter: string) =>
  filterCatalog(catalog, filter).map(node => [
    node.schema.name,
    node.tables.map(table => [table.table.name, table.columns.map(column => column.name)]),
  ]);

describe('filterCatalog', () => {
  it('lists everything without a filter', () => {
    expect(names('')).toEqual([
      [
        'RETAIL',
        [
          ['CUSTOMERS', ['CUSTOMER_ID']],
          ['REGIONS', ['REGION_ID', 'REGION_NAME']],
          ['SALES', ['SALE_ID', 'REGION_ID']],
        ],
      ],
      ['STAGING', []],
    ]);
  });

  it('keeps the tables and columns whose names contain the filter', () => {
    expect(names('region')).toEqual([
      [
        'RETAIL',
        [
          ['REGIONS', ['REGION_ID', 'REGION_NAME']],
          ['SALES', ['REGION_ID']],
        ],
      ],
    ]);
    expect(filterCatalog(catalog, 'region')[0].matched).toBe(true);
  });

  it('keeps a matching schema with all its tables', () => {
    expect(names('stag')).toEqual([['STAGING', []]]);
  });
});

describe('sqlName', () => {
  it('quotes names that are not regular identifiers', () => {
    expect(sqlName('REGION_ID')).toBe('REGION_ID');
    expect(sqlName('accident_id')).toBe('"accident_id"');
    expect(sqlName('ORDER')).toBe('"ORDER"');
    expect(sqlName('a"b')).toBe('"a""b"');
  });
});
