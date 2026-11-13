import type { Completion, CompletionContext, CompletionResult } from '@codemirror/autocomplete';
import type { Catalog, TableInfo } from './catalog';
import { splitStatements, statementAt } from './splitter';

const unquote = (name: string) => (name.startsWith('"') ? name.slice(1, -1).replaceAll('""', '"') : name.toUpperCase());

/** Tables that a statement names as `SCHEMA.TABLE`, or by name alone in the open schema. */
export function tablesNamedIn(sql: string, catalog: Catalog): TableInfo[] {
  const found = new Set<TableInfo>();
  for (const match of sql.matchAll(/("(?:[^"]|"")+"|[A-Za-z_][\w$#]*)\s*\.\s*("(?:[^"]|"")+"|[A-Za-z_][\w$#]*)/g)) {
    const schema = catalog.schemas.find(candidate => candidate.name === unquote(match[1]!));
    const table = schema?.tables.find(candidate => candidate.name === unquote(match[2]!));
    if (table) found.add(table);
  }
  return [...found];
}

/**
 * Completion of the database's own names: schemas, a schema's tables and views after `SCHEMA.`, a table's
 * columns after `SCHEMA.TABLE.`, and the columns of the tables that the statement at the cursor names.
 * `onSchemasNeeded` asks for the columns of schemas the statement names that have not been read yet.
 */
export function namesCompletion(getCatalog: () => Catalog | undefined, onSchemasNeeded: (schemas: string[]) => void) {
  return (context: CompletionContext): CompletionResult | null => {
    const catalog = getCatalog();
    if (!catalog) return null;
    const word = context.matchBefore(
      /(?:(?:"(?:[^"]|"")*"|[A-Za-z_][\w$#]*)\s*\.\s*){0,2}(?:"(?:[^"]|"")*"?|[\w$#]*)$/
    );
    if (!word || (word.from === word.to && !context.explicit)) return null;

    const text = context.state.doc.toString();
    const statement = statementAt(splitStatements(text), context.pos, text);
    const statementText = statement?.text ?? '';
    const parts = word.text.split(/\s*\.\s*/);
    const prefixFrom = word.to - parts.at(-1)!.length;

    const missing = new Set<string>();
    const need = (schema: string) => {
      const info = catalog.schemas.find(candidate => candidate.name === schema);
      if (info && !info.columnsLoaded) missing.add(schema);
    };

    let options: Completion[] = [];
    if (parts.length === 1) {
      const tables = tablesNamedIn(statementText, catalog);
      for (const match of statementText.matchAll(/("?)([A-Za-z_][\w$#]*)\1\s*\./g))
        need(match[1] ? match[2]! : match[2]!.toUpperCase());
      options = [
        ...catalog.schemas.map(schema => ({
          label: schema.name,
          type: 'namespace',
          detail: schema.virtual ? 'virtual schema' : 'schema',
          boost: -1,
        })),
        ...tables.flatMap(table =>
          table.columns.map(column => ({
            label: column.name,
            type: 'property',
            detail: `${table.name} · ${column.type}`,
            boost: 2,
          }))
        ),
      ];
    } else if (parts.length === 2) {
      const schema = catalog.schemas.find(candidate => candidate.name === unquote(parts[0]!));
      if (schema) {
        need(schema.name);
        options = schema.tables.map(table => ({
          label: table.name,
          type: table.kind === 'view' ? 'class' : 'type',
          detail: table.kind,
        }));
      } else {
        // An alias or a table of the open schema: offer the columns of the statement's tables.
        options = tablesNamedIn(statementText, catalog).flatMap(table =>
          table.columns.map(column => ({ label: column.name, type: 'property', detail: column.type }))
        );
      }
    } else {
      const schema = catalog.schemas.find(candidate => candidate.name === unquote(parts[0]!));
      const table = schema?.tables.find(candidate => candidate.name === unquote(parts[1]!));
      if (schema) need(schema.name);
      options = (table?.columns ?? []).map(column => ({ label: column.name, type: 'property', detail: column.type }));
    }

    if (missing.size > 0) onSchemasNeeded([...missing]);
    if (options.length === 0) return null;
    return { from: prefixFrom, options, validFor: /^[\w$#]*$/ };
  };
}
