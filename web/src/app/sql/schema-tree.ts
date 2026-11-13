import { Catalog, ColumnInfo, SchemaInfo, TableInfo } from './catalog';
import { KEYWORDS } from './dialect';

export interface TableNode {
  table: TableInfo;
  /** The columns to list when the table is expanded. */
  columns: ColumnInfo[];
  /** Whether the filter matched some of the table's columns, which expands it. */
  matched: boolean;
}

export interface SchemaNode {
  schema: SchemaInfo;
  tables: TableNode[];
  /** Whether the filter matched something inside the schema, which expands it. */
  matched: boolean;
}

/**
 * The schemas, tables, and columns whose names contain the filter, ignoring case. A schema whose name matches
 * keeps all its tables; a table whose name matches keeps all its columns.
 */
export function filterCatalog(catalog: Catalog, filter: string): SchemaNode[] {
  const needle = filter.trim().toUpperCase();
  const contains = (name: string) => name.toUpperCase().includes(needle);
  const all = (schema: SchemaInfo): SchemaNode => ({
    schema,
    tables: schema.tables.map(table => ({ table, columns: table.columns, matched: false })),
    matched: false,
  });
  if (!needle) return catalog.schemas.map(all);

  return catalog.schemas.flatMap(schema => {
    if (contains(schema.name)) return [all(schema)];
    const tables = schema.tables.flatMap((table): TableNode[] => {
      const columns = table.columns.filter(column => contains(column.name));
      if (columns.length > 0) return [{ table, columns, matched: true }];
      return contains(table.name) ? [{ table, columns: table.columns, matched: false }] : [];
    });
    return tables.length > 0 ? [{ schema, tables, matched: true }] : [];
  });
}

const RESERVED = new Set(KEYWORDS.map(word => word.toUpperCase()));

/** The name as SQL writes it: plain when it is a regular identifier, else quoted. */
export function sqlName(name: string): string {
  return /^[A-Z][A-Z0-9_]*$/.test(name) && !RESERVED.has(name) ? name : `"${name.replaceAll('"', '""')}"`;
}
