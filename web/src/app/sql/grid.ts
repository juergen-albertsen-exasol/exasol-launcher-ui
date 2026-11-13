import { ColDef, ValueFormatterParams, ValueGetterParams } from '@ag-grid-community/core';
import { SqlColumn } from '@exa/services/local-api/local-api.models';

export type Row = (string | null)[];

const NUMERIC = /^(DECIMAL|DOUBLE)\b/;

const escapeHtml = (text: string) => text.replace(/[&<>"']/g, char => `&#${char.charCodeAt(0)};`);

// AG Grid's header template; the eText element receives the column's name.
const headerTemplate = (type: string) =>
  '<div class="ag-cell-label-container" role="presentation">' +
  '<div ref="eLabel" class="ag-header-cell-label sql-grid__header" role="presentation">' +
  '<span ref="eText" class="ag-header-cell-text"></span>' +
  `<small class="sql-grid__type">${escapeHtml(type)}</small>` +
  '</div></div>';

/** The grid's columns: a row number, then each result column with its type, NULL marked, numbers aligned right. */
export function columnDefs(columns: SqlColumn[]): ColDef<Row>[] {
  return [
    {
      headerName: '#',
      colId: '#',
      valueGetter: (params: ValueGetterParams<Row>) => (params.node?.rowIndex ?? 0) + 1,
      width: 72,
      pinned: 'left',
      cellClass: 'sql-grid__row-number',
      type: 'rightAligned',
    },
    ...columns.map(
      (column, index): ColDef<Row> => ({
        headerName: column.name,
        colId: String(index),
        headerComponentParams: { template: headerTemplate(column.type) },
        valueGetter: (params: ValueGetterParams<Row>) => params.data?.[index] ?? null,
        valueFormatter: (params: ValueFormatterParams<Row>) => (params.value === null ? 'NULL' : String(params.value)),
        cellClassRules: { 'sql-grid__null': params => params.value === null },
        type: NUMERIC.test(column.type) ? 'rightAligned' : undefined,
      })
    ),
  ];
}
