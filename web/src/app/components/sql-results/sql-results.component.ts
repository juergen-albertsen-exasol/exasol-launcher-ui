import { Component, ElementRef, computed, inject, input, output } from '@angular/core';
import { AgGridModule } from '@ag-grid-community/angular';
import { ClientSideRowModelModule } from '@ag-grid-community/client-side-row-model';
import { AgGridEvent, ColDef, ModuleRegistry } from '@ag-grid-community/core';
import { Row, columnDefs } from '@exa/sql/grid';
import { formatCount, formatRows, resultSummary, tabLabel } from '@exa/sql/labels';
import { ResultTab } from '@exa/store/sql/sql.state';

ModuleRegistry.registerModules([ClientSideRowModelModule]);

const isMac = () => typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

/** The result tabs of a SQL page: each statement's rows, row count, or error. */
@Component({
  selector: 'exa-sql-results',
  templateUrl: './sql-results.component.html',
  styleUrls: ['./sql-results.component.scss'],
  imports: [AgGridModule],
})
export class SqlResultsComponent {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly tabs = input.required<ResultTab[]>();
  readonly selected = input(0);
  /** Milliseconds since the epoch, for the time a statement has been running. */
  readonly now = input(0);
  readonly stopped = input(false);
  /** The button text that moves the editor's cursor to the selected tab's error, when it names a position. */
  readonly goToLabel = input<string | undefined>(undefined);

  readonly selectTab = output<number>();
  readonly goTo = output<number>();
  readonly download = output<number>();

  readonly run = isMac() ? '⌘↵' : 'Ctrl+↵';
  readonly runAll = isMac() ? '⇧⌘↵' : 'Shift+Ctrl+↵';
  readonly label = tabLabel;
  readonly summary = resultSummary;
  readonly formatRows = formatRows;
  readonly formatCount = formatCount;

  readonly tab = computed<ResultTab | undefined>(() => this.tabs()[this.selected()]);
  readonly outcome = computed(() => this.tab()?.outcome);
  readonly resultSet = computed(() => {
    const outcome = this.outcome();
    return outcome?.type === 'result_set' ? outcome : undefined;
  });
  readonly affected = computed(() => {
    const outcome = this.outcome();
    return outcome?.type === 'row_count' ? outcome.row_count : 0;
  });
  readonly columns = computed<ColDef<Row>[]>(() => columnDefs(this.resultSet()?.columns ?? []));
  readonly errorText = computed(() => {
    const error = this.tab()?.error;
    if (!error) return '';
    return error.code ? `[${error.code}] ${error.detail}` : error.detail ?? '';
  });
  /** The statement that Run all did not reach because the selected one failed. */
  readonly stoppedBefore = computed(() => {
    const next = this.tabs()[this.selected() + 1];
    return next?.status === 'not_run' ? this.selected() + 2 : undefined;
  });

  /** Sizes the columns to their content, and stretches them when they leave the grid's width unused. */
  fitColumns(event: AgGridEvent<Row>): void {
    event.columnApi.autoSizeAllColumns();
    const used = event.columnApi.getAllDisplayedColumns().reduce((width, column) => width + column.getActualWidth(), 0);
    const viewport = this.host.nativeElement.querySelector('.ag-body-viewport');
    if (viewport && used < viewport.clientWidth) event.api.sizeColumnsToFit();
  }

  copyError(): void {
    navigator.clipboard?.writeText(this.errorText()).catch(() => undefined);
  }
}
