import {
  Component,
  DestroyRef,
  NgZone,
  computed,
  effect,
  inject,
  input,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { Store } from '@ngxs/store';
import { ErrorNotificationsAction, ExaButtonComponent } from '@exasol/ui-common';
import { SqlEditorComponent } from '@exa/components/sql-editor/sql-editor.component';
import { SqlResultsComponent } from '@exa/components/sql-results/sql-results.component';
import {
  DEPLOYMENT_READY_STATUS,
  Operation,
  OperationKind,
  OperationStatus,
} from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import { formatSql } from '@exa/sql/format';
import { formatElapsed, lineAndColumn } from '@exa/sql/labels';
import { Statement, positionInStatement, splitStatements, statementAt } from '@exa/sql/splitter';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { RUNNING_VERB } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import {
  CancelStatementsAction,
  CloseSqlSessionAction,
  RunStatementsAction,
  SelectResultTabAction,
} from '@exa/store/sql/sql.actions';
import { ResultTab, SqlState, StatementToRun, emptySqlPage } from '@exa/store/sql/sql.state';

const TICK_MS = 1000;

const isMac = () => typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

const toRun = (statement: Statement, offset = 0): StatementToRun => ({
  sql: statement.text,
  from: statement.from + offset,
  to: statement.to + offset,
});

/** The editor offset that a failed tab's error names, when it names one. */
const errorOffset = (tab: ResultTab | undefined): number | undefined => {
  const error = tab?.status === 'failed' ? tab.error : undefined;
  if (!tab || !error?.line || !error.column) return undefined;
  return positionInStatement({ text: tab.sql, from: tab.from }, error.line, error.column);
};

@Component({
  selector: 'exa-page-sql',
  templateUrl: './page-sql.component.html',
  styleUrls: ['./page-sql.component.scss'],
  imports: [ExaButtonComponent, SqlEditorComponent, SqlResultsComponent],
})
export class PageSqlComponent {
  private readonly store = inject(Store);
  private readonly api = inject(LocalApiService);
  private readonly zone = inject(NgZone);

  readonly name = input.required<string>();

  private readonly editor = viewChild.required(SqlEditorComponent);
  private readonly pages = this.store.selectSignal(SqlState.pages);
  private readonly summary = this.store.selectSignal(DeploymentsState.selectedSummary);
  private readonly operations = this.store.selectSignal(OperationsState.operations);

  readonly page = computed(() => this.pages()[this.name()] ?? emptySqlPage());
  readonly ready = computed(() => this.summary()?.status === DEPLOYMENT_READY_STATUS);
  readonly stopped = computed(() => !!this.summary() && !this.ready());
  readonly operation = computed(() => {
    const latest = this.operations().find(operation => operation.deployment === this.name());
    return latest?.status === OperationStatus.RUNNING ? latest : undefined;
  });
  readonly canStart = computed(() => !this.operation() && this.summary()?.status === 'stopped');
  readonly operationVerb = RUNNING_VERB;

  readonly runKey = isMac() ? '⌘↵' : 'Ctrl+↵';
  readonly runAllKey = isMac() ? '⇧⌘↵' : 'Shift+Ctrl+↵';
  readonly now = signal(Date.now());

  readonly runningIndex = computed(() => this.page().tabs.findIndex(tab => tab.status === 'running'));
  readonly elapsed = computed(() => {
    const startedAt = this.page().tabs[this.runningIndex()]?.startedAt;
    return formatElapsed(startedAt === undefined ? 0 : this.now() - startedAt);
  });
  readonly selectedTab = computed(() => this.page().tabs[this.page().selected]);
  private readonly text = signal('');
  readonly goToLabel = computed(() => {
    const offset = errorOffset(this.selectedTab());
    if (offset === undefined) return undefined;
    const { line, column } = lineAndColumn(this.text(), offset);
    return `Go to line ${line}, column ${column}`;
  });

  constructor() {
    effect(() => {
      const name = this.name();
      untracked(() => this.store.dispatch(new SelectDeploymentAction(name)));
    });
    effect(onCleanup => {
      const name = this.name();
      onCleanup(() => this.store.dispatch(new CloseSqlSessionAction(name)));
    });
    effect(() => {
      const tab = this.page().tabs[this.runningIndex()];
      untracked(() => this.editor().markRunning(tab && { from: tab.from, to: tab.to }));
    });
    effect(() => {
      const offset = errorOffset(this.selectedTab());
      untracked(() => this.editor().markError(offset));
    });
    this.tickWhileRunning();
    this.refreshWhenOperationEnds();

    const closeOnHide = () => this.store.dispatch(new CloseSqlSessionAction(this.name()));
    window.addEventListener('pagehide', closeOnHide);
    inject(DestroyRef).onDestroy(() => window.removeEventListener('pagehide', closeOnHide));
  }

  textChanged(text: string): void {
    this.text.set(text);
  }

  /** Runs the selected statements, or the statement at the cursor. */
  run(): void {
    if (!this.ready() || this.page().running) return;
    const editor = this.editor();
    const selection = editor.selection();
    if (selection) {
      this.dispatchRun(splitStatements(selection.text).map(statement => toRun(statement, selection.from)));
      return;
    }
    const text = editor.text();
    const statement = statementAt(splitStatements(text), editor.cursor(), text);
    if (statement) this.dispatchRun([toRun(statement)]);
  }

  runAll(): void {
    if (!this.ready() || this.page().running) return;
    this.dispatchRun(splitStatements(this.editor().text()).map(statement => toRun(statement)));
  }

  cancel(): void {
    this.store.dispatch(new CancelStatementsAction(this.name()));
  }

  /** Formats each statement in place, keeping the text between statements. */
  format(): void {
    const editor = this.editor();
    const text = editor.text();
    let formatted = text;
    for (const statement of splitStatements(text).reverse()) {
      formatted = formatted.slice(0, statement.from) + formatSql(statement.text) + formatted.slice(statement.to);
    }
    if (formatted !== text) editor.replaceAll(formatted);
  }

  selectTab(index: number): void {
    this.store.dispatch(new SelectResultTabAction(this.name(), index));
  }

  goTo(index: number): void {
    const offset = errorOffset(this.page().tabs[index]);
    if (offset !== undefined) this.editor().select(offset);
  }

  download(index: number): void {
    const name = this.name();
    const { sessionId, tabs } = this.page();
    const tab = tabs[index];
    if (!sessionId || !tab) return;
    this.api.createExport(name, sessionId, tab.sql).subscribe({
      next: url => {
        const link = document.createElement('a');
        link.href = url;
        link.download = `${name}-result.csv`;
        link.click();
      },
      error: (error: unknown) =>
        this.store.dispatch(new ErrorNotificationsAction(`Cannot download the result: ${problemDetail(error)}`)),
    });
  }

  start(): void {
    this.store.dispatch(new RunOperationAction(this.name(), OperationKind.START));
  }

  private dispatchRun(statements: StatementToRun[]): void {
    if (statements.length > 0) this.store.dispatch(new RunStatementsAction(this.name(), statements));
  }

  // The timer runs outside Angular's zone, so pages can still become stable.
  private tickWhileRunning(): void {
    effect(onCleanup => {
      if (!this.page().running) return;
      this.now.set(Date.now());
      const timer = this.zone.runOutsideAngular(() =>
        setInterval(() => this.zone.run(() => this.now.set(Date.now())), TICK_MS)
      );
      onCleanup(() => clearInterval(timer));
    });
  }

  private refreshWhenOperationEnds(): void {
    let previous: Operation | undefined;
    effect(() => {
      const operation = this.operation();
      if (previous && !operation) untracked(() => this.store.dispatch(new LoadDeploymentsAction(true)));
      previous = operation;
    });
  }
}
