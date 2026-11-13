import { HttpErrorResponse, HttpStatusCode } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Action, Selector, State, StateContext } from '@ngxs/store';
import { patch, updateItem } from '@ngxs/store/operators';
import { firstValueFrom } from 'rxjs';
import { StatementOutcome, StatementProblem } from '@exa/services/local-api/local-api.models';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';
import {
  CancelStatementsAction,
  CloseSqlSessionAction,
  RunStatementsAction,
  SelectResultTabAction,
} from './sql.actions';

/** The database's code for a statement that was aborted. */
export const ABORTED_CODE = 'R0003';

export interface StatementToRun {
  sql: string;
  /** Offsets of the statement in the editor's text. */
  from: number;
  to: number;
}

export type ResultStatus = 'waiting' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'not_run';

export interface ResultTab extends StatementToRun {
  status: ResultStatus;
  outcome?: StatementOutcome;
  error?: StatementProblem;
  /** When the statement started, in milliseconds since the epoch. */
  startedAt?: number;
}

export interface SqlPage {
  sessionId?: string;
  schema: string | null;
  tabs: ResultTab[];
  selected: number;
  running: boolean;
  cancelling: boolean;
  /** A new session replaced one that ended, which reset the open schema and any transaction. */
  sessionRenewed: boolean;
}

export interface SqlStateModel {
  pages: { [name: string]: SqlPage };
}

export const emptySqlPage = (): SqlPage => ({
  schema: null,
  tabs: [],
  selected: 0,
  running: false,
  cancelling: false,
  sessionRenewed: false,
});

const asProblem = (error: unknown): StatementProblem => {
  if (error instanceof HttpErrorResponse && error.error && typeof error.error === 'object') {
    const problem = error.error as StatementProblem;
    if (problem.detail) return problem;
  }
  return { type: 'about:blank', title: 'Error', detail: problemDetail(error) };
};

const isNotFound = (error: unknown) => error instanceof HttpErrorResponse && error.status === HttpStatusCode.NotFound;

@State<SqlStateModel>({ name: 'sql', defaults: { pages: {} } })
@Injectable()
export class SqlState {
  private readonly api = inject(LocalApiService);

  @Selector()
  static pages(state: SqlStateModel): SqlStateModel['pages'] {
    return state.pages;
  }

  @Action(RunStatementsAction)
  async run(ctx: StateContext<SqlStateModel>, { name, statements }: RunStatementsAction): Promise<void> {
    if (statements.length === 0 || this.page(ctx, name).running) return;
    this.update(ctx, name, {
      tabs: statements.map(statement => ({ ...statement, status: 'waiting' })),
      selected: 0,
      running: true,
      cancelling: false,
      sessionRenewed: false,
    });

    for (let index = 0; index < statements.length; index++) {
      if (this.page(ctx, name).cancelling) {
        this.markRestNotRun(ctx, name, index);
        break;
      }
      this.updateTab(ctx, name, index, { status: 'running', startedAt: Date.now() });
      const succeeded = await this.runOne(ctx, name, index, statements[index].sql);
      if (!succeeded) {
        this.markRestNotRun(ctx, name, index + 1);
        break;
      }
    }
    this.update(ctx, name, { running: false, cancelling: false });
  }

  @Action(CancelStatementsAction)
  async cancel(ctx: StateContext<SqlStateModel>, { name }: CancelStatementsAction): Promise<void> {
    const page = this.page(ctx, name);
    if (!page.running) return;
    this.update(ctx, name, { cancelling: true });
    if (page.sessionId) {
      await firstValueFrom(this.api.cancelStatement(name, page.sessionId)).catch(() => undefined);
    }
  }

  @Action(SelectResultTabAction)
  select(ctx: StateContext<SqlStateModel>, { name, index }: SelectResultTabAction): void {
    this.update(ctx, name, { selected: index });
  }

  @Action(CloseSqlSessionAction)
  close(ctx: StateContext<SqlStateModel>, { name }: CloseSqlSessionAction): void {
    const sessionId = this.page(ctx, name).sessionId;
    if (!sessionId) return;
    this.api.closeSqlSession(name, sessionId);
    this.update(ctx, name, { sessionId: undefined, schema: null });
  }

  private async runOne(ctx: StateContext<SqlStateModel>, name: string, index: number, sql: string): Promise<boolean> {
    try {
      const outcome = await this.execute(ctx, name, sql);
      this.updateTab(ctx, name, index, { status: 'succeeded', outcome });
      this.update(ctx, name, { schema: outcome.schema, selected: index });
      return true;
    } catch (error) {
      const problem = asProblem(error);
      const cancelled = this.page(ctx, name).cancelling && problem.code === ABORTED_CODE;
      this.updateTab(ctx, name, index, { status: cancelled ? 'cancelled' : 'failed', error: problem });
      this.update(ctx, name, { selected: index });
      return false;
    }
  }

  /** Runs sql in the page's session, opening one first, and once more in a new session when it ended. */
  private async execute(ctx: StateContext<SqlStateModel>, name: string, sql: string): Promise<StatementOutcome> {
    const existing = this.page(ctx, name).sessionId;
    const sessionId = existing ?? (await this.openSession(ctx, name));
    try {
      return await firstValueFrom(this.api.runStatement(name, sessionId, sql));
    } catch (error) {
      if (!existing || !isNotFound(error)) throw error;
      const renewed = await this.openSession(ctx, name);
      this.update(ctx, name, { sessionRenewed: true, schema: null });
      return await firstValueFrom(this.api.runStatement(name, renewed, sql));
    }
  }

  private async openSession(ctx: StateContext<SqlStateModel>, name: string): Promise<string> {
    const { id } = await firstValueFrom(this.api.openSqlSession(name));
    this.update(ctx, name, { sessionId: id });
    return id;
  }

  private markRestNotRun(ctx: StateContext<SqlStateModel>, name: string, from: number): void {
    const tabs = this.page(ctx, name).tabs;
    for (let index = from; index < tabs.length; index++) this.updateTab(ctx, name, index, { status: 'not_run' });
  }

  private page(ctx: StateContext<SqlStateModel>, name: string): SqlPage {
    return ctx.getState().pages[name] ?? emptySqlPage();
  }

  private update(ctx: StateContext<SqlStateModel>, name: string, changes: Partial<SqlPage>): void {
    ctx.setState(patch<SqlStateModel>({ pages: patch({ [name]: { ...this.page(ctx, name), ...changes } }) }));
  }

  private updateTab(ctx: StateContext<SqlStateModel>, name: string, index: number, changes: Partial<ResultTab>): void {
    const tabs = this.page(ctx, name).tabs;
    this.update(ctx, name, {
      tabs: updateItem<ResultTab>(index, { ...tabs[index], ...changes })(tabs),
    });
  }
}
