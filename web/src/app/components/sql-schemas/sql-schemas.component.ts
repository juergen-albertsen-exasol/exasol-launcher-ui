import { Component, computed, effect, input, output, signal, untracked } from '@angular/core';
import { Catalog } from '@exa/sql/catalog';
import { SchemaNode, TableNode, filterCatalog, sqlName } from '@exa/sql/schema-tree';

const tableKey = (schema: string, table: string) => `${schema}.${table}`;

/** The database's schemas, tables, and columns, with a filter; double-clicking a name inserts it. */
@Component({
  selector: 'exa-sql-schemas',
  templateUrl: './sql-schemas.component.html',
  styleUrls: ['./sql-schemas.component.scss'],
})
export class SqlSchemasComponent {
  readonly catalog = input<Catalog | undefined>(undefined);
  readonly openSchema = input<string | null>(null);
  readonly loading = input(false);
  readonly error = input<string | undefined>(undefined);
  readonly stopped = input(false);

  readonly insert = output<string>();
  readonly loadColumns = output<string[]>();
  readonly reload = output<void>();

  readonly filter = signal('');
  private readonly expandedSchemas = signal(new Set<string>());
  private readonly expandedTables = signal(new Set<string>());
  private readonly requested = new Set<string>();

  readonly nodes = computed(() => {
    const catalog = this.catalog();
    return catalog ? filterCatalog(catalog, this.filter()) : [];
  });

  constructor() {
    // Filtering by column names needs every schema's columns.
    effect(() => {
      if (!this.filter().trim()) return;
      const missing = (this.catalog()?.schemas ?? [])
        .filter(schema => !schema.columnsLoaded && !this.requested.has(schema.name))
        .map(schema => schema.name);
      if (missing.length === 0) return;
      missing.forEach(schema => this.requested.add(schema));
      untracked(() => this.loadColumns.emit(missing));
    });
  }

  schemaExpanded(node: SchemaNode): boolean {
    return this.expandedSchemas().has(node.schema.name) || (!!this.filter().trim() && node.matched);
  }

  tableExpanded(schema: SchemaNode, node: TableNode): boolean {
    return (
      this.expandedTables().has(tableKey(schema.schema.name, node.table.name)) ||
      (!!this.filter().trim() && node.matched)
    );
  }

  toggleSchema(node: SchemaNode): void {
    this.expandedSchemas.update(expanded => toggled(expanded, node.schema.name));
  }

  toggleTable(schema: SchemaNode, node: TableNode): void {
    const key = tableKey(schema.schema.name, node.table.name);
    const expanding = !this.expandedTables().has(key);
    this.expandedTables.update(expanded => toggled(expanded, key));
    if (expanding && !schema.schema.columnsLoaded && !this.requested.has(schema.schema.name)) {
      this.requested.add(schema.schema.name);
      this.loadColumns.emit([schema.schema.name]);
    }
  }

  insertName(name: string): void {
    this.insert.emit(sqlName(name));
  }

  filterChanged(event: Event): void {
    this.filter.set((event.target as HTMLInputElement).value);
  }
}

function toggled(set: Set<string>, key: string): Set<string> {
  const next = new Set(set);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  return next;
}
