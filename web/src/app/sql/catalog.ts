/** The database's own names, as the schema endpoints report them. */
export interface Catalog {
  schemas: SchemaInfo[];
}

export interface SchemaInfo {
  name: string;
  virtual: boolean;
  tables: TableInfo[];
  /** Whether the columns of this schema's tables have been read. */
  columnsLoaded: boolean;
}

export interface TableInfo {
  name: string;
  kind: 'table' | 'view';
  columns: ColumnInfo[];
}

export interface ColumnInfo {
  name: string;
  type: string;
}

/** Statements after which the schema list and completion read the names again. */
export const DDL_KEYWORDS = new Set(['CREATE', 'ALTER', 'DROP', 'RENAME', 'OPEN']);
