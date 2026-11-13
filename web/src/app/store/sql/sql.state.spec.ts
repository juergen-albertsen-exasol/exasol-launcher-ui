import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Store, provideStore } from '@ngxs/store';
import { Observable, Subject, firstValueFrom, of, throwError } from 'rxjs';
import { SchemaSummary, StatementOutcome } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import {
  CancelStatementsAction,
  CloseSqlSessionAction,
  LoadSchemaColumnsAction,
  LoadSchemasAction,
  RunStatementsAction,
  SelectResultTabAction,
} from './sql.actions';
import { SqlPage, SqlState, StatementToRun } from './sql.state';

const rows = (schema: string | null = null): StatementOutcome => ({
  type: 'result_set',
  columns: [{ name: 'N', type: 'DECIMAL(18,0)' }],
  rows: [['1']],
  total_rows: 1,
  truncated: false,
  duration_ms: 3,
  schema,
});

const retail = (tables = ['REGIONS', 'SALES']): SchemaSummary[] => [
  { name: 'RETAIL', virtual: false, objects: tables.map(name => ({ name, kind: 'table' as const })) },
];

const retailColumns = (): SchemaSummary => ({
  name: 'RETAIL',
  virtual: false,
  objects: [{ name: 'REGIONS', kind: 'table', columns: [{ name: 'REGION_ID', type: 'DECIMAL(9,0)' }] }],
});

const until = async (condition: () => boolean) => {
  for (let attempt = 0; attempt < 50 && !condition(); attempt++) await new Promise(resolve => setTimeout(resolve));
};

const problem = (status: number, body: object) => new HttpErrorResponse({ status, error: body });

const statements = (...texts: string[]): StatementToRun[] => {
  let offset = 0;
  return texts.map(sql => {
    const statement = { sql, from: offset, to: offset + sql.length };
    offset += sql.length + 2;
    return statement;
  });
};

describe('SqlState', () => {
  let store: Store;
  let api: {
    openSqlSession: jest.Mock;
    runStatement: jest.Mock<Observable<StatementOutcome>, [string, string, string]>;
    cancelStatement: jest.Mock;
    closeSqlSession: jest.Mock;
    listSchemas: jest.Mock;
    getSchema: jest.Mock;
  };

  beforeEach(() => {
    let sessions = 0;
    api = {
      openSqlSession: jest.fn(() => of({ id: `s${++sessions}` })),
      runStatement: jest.fn<Observable<StatementOutcome>, [string, string, string]>(() => of(rows())),
      cancelStatement: jest.fn(() => of(undefined)),
      closeSqlSession: jest.fn(),
      listSchemas: jest.fn(() => of(retail())),
      getSchema: jest.fn(() => of(retailColumns())),
    };
    TestBed.configureTestingModule({
      providers: [provideStore([SqlState]), { provide: LocalApiService, useValue: api }],
    });
    store = TestBed.inject(Store);
  });

  const page = (name = 'default'): SqlPage => store.selectSnapshot(SqlState.pages)[name];
  const run = (...texts: string[]) =>
    firstValueFrom(store.dispatch(new RunStatementsAction('default', statements(...texts))));

  it('opens a session for the first statement and reports the open schema', async () => {
    api.runStatement.mockReturnValueOnce(of(rows('RETAIL')));

    await run('OPEN SCHEMA RETAIL');

    expect(api.openSqlSession).toHaveBeenCalledWith('default');
    expect(api.runStatement).toHaveBeenCalledWith('default', 's1', 'OPEN SCHEMA RETAIL');
    expect(page()).toMatchObject({ sessionId: 's1', schema: 'RETAIL', running: false, selected: 0 });
    expect(page().tabs).toEqual([expect.objectContaining({ status: 'succeeded', outcome: rows('RETAIL') })]);
  });

  it('runs statements in order in one session, each with its own tab', async () => {
    await run('OPEN SCHEMA RETAIL', 'SELECT 1');

    expect(api.openSqlSession).toHaveBeenCalledTimes(1);
    expect(api.runStatement.mock.calls.map(call => call[2])).toEqual(['OPEN SCHEMA RETAIL', 'SELECT 1']);
    expect(page().tabs.map(tab => tab.status)).toEqual(['succeeded', 'succeeded']);
    expect(page().selected).toBe(1);
  });

  it('stops at the first failure and marks the rest as not run', async () => {
    const failure = { type: 'about:blank', title: 'Unprocessable Entity', detail: 'object X not found', code: '42000' };
    api.runStatement.mockReturnValueOnce(of(rows())).mockReturnValueOnce(throwError(() => problem(422, failure)));

    await run('SELECT 1', 'SELECT X', 'SELECT 3');

    expect(api.runStatement).toHaveBeenCalledTimes(2);
    expect(page().tabs.map(tab => tab.status)).toEqual(['succeeded', 'failed', 'not_run']);
    expect(page().tabs[1].error).toEqual(failure);
    expect(page().selected).toBe(1);
  });

  it('runs a statement in a new session when its session ended', async () => {
    await run('SELECT 1');
    api.runStatement
      .mockReturnValueOnce(throwError(() => problem(404, { detail: 'the SQL session does not exist' })))
      .mockReturnValueOnce(of(rows()));

    await run('SELECT 2');

    expect(api.runStatement.mock.calls.slice(1).map(call => call[1])).toEqual(['s1', 's2']);
    expect(page()).toMatchObject({ sessionId: 's2', sessionRenewed: true });
    expect(page().tabs[0].status).toBe('succeeded');

    await run('SELECT 3');
    expect(page().sessionRenewed).toBe(false);
  });

  it('reports a database that is not running on the statement', async () => {
    api.openSqlSession.mockReturnValueOnce(
      throwError(() => problem(409, { detail: 'the database of default is not running' }))
    );

    await run('SELECT 1', 'SELECT 2');

    expect(page().tabs.map(tab => tab.status)).toEqual(['failed', 'not_run']);
    expect(page().tabs[0].error?.detail).toBe('the database of default is not running');
  });

  it('cancels the running statement and leaves the waiting ones not run', async () => {
    const second = new Subject<StatementOutcome>();
    api.runStatement.mockReturnValueOnce(of(rows())).mockReturnValueOnce(second);

    const running = run('SELECT 1', 'SELECT SLOW', 'SELECT 3');
    await until(() => page()?.tabs[1]?.status === 'running');
    expect(page().tabs.map(tab => tab.status)).toEqual(['succeeded', 'running', 'waiting']);

    await firstValueFrom(store.dispatch(new CancelStatementsAction('default')));
    second.error(problem(422, { detail: 'Client requested execution abort.', code: 'R0003' }));
    await running;

    expect(api.cancelStatement).toHaveBeenCalledWith('default', 's1');
    expect(page().tabs.map(tab => tab.status)).toEqual(['succeeded', 'cancelled', 'not_run']);
    expect(page()).toMatchObject({ running: false, cancelling: false });
  });

  it('selects a tab and closes the session', async () => {
    await run('SELECT 1', 'SELECT 2');

    await firstValueFrom(store.dispatch(new SelectResultTabAction('default', 0)));
    await firstValueFrom(store.dispatch(new CloseSqlSessionAction('default')));

    expect(page()).toMatchObject({ selected: 0, sessionId: undefined, schema: null });
    expect(api.closeSqlSession).toHaveBeenCalledWith('default', 's1');
  });

  it('loads the schemas and the columns of a schema', async () => {
    await firstValueFrom(store.dispatch(new LoadSchemasAction('default')));
    expect(page().catalog).toEqual({
      schemas: [
        {
          name: 'RETAIL',
          virtual: false,
          columnsLoaded: false,
          tables: [
            { name: 'REGIONS', kind: 'table', columns: [] },
            { name: 'SALES', kind: 'table', columns: [] },
          ],
        },
      ],
    });

    await firstValueFrom(store.dispatch(new LoadSchemaColumnsAction('default', ['RETAIL'])));

    expect(api.getSchema).toHaveBeenCalledWith('default', 'RETAIL');
    expect(page().catalog?.schemas[0]).toMatchObject({
      columnsLoaded: true,
      tables: [{ name: 'REGIONS', columns: [{ name: 'REGION_ID', type: 'DECIMAL(9,0)' }] }],
    });
  });

  it('reports a schema listing that fails', async () => {
    api.listSchemas.mockReturnValueOnce(throwError(() => problem(409, { detail: 'the database is not running' })));

    await firstValueFrom(store.dispatch(new LoadSchemasAction('default')));

    expect(page()).toMatchObject({ catalogError: 'the database is not running', catalogLoading: false });
  });

  it('reads the names again after a statement that changes them', async () => {
    await firstValueFrom(store.dispatch(new LoadSchemasAction('default')));
    await firstValueFrom(store.dispatch(new LoadSchemaColumnsAction('default', ['RETAIL'])));
    api.listSchemas.mockReturnValueOnce(of(retail(['REGIONS', 'RETURNS', 'SALES'])));
    api.getSchema.mockReturnValueOnce(
      of({
        ...retailColumns(),
        objects: [
          ...retailColumns().objects,
          { name: 'RETURNS', kind: 'table', columns: [{ name: 'ID', type: 'DECIMAL(18,0)' }] },
        ],
      })
    );

    await run('SELECT 1');
    expect(api.listSchemas).toHaveBeenCalledTimes(1);

    await run('-- returns\ncreate table RETAIL.RETURNS (ID INT)');

    expect(api.listSchemas).toHaveBeenCalledTimes(2);
    expect(api.getSchema).toHaveBeenCalledTimes(2);
    expect(page().catalog?.schemas[0].tables.map(table => table.name)).toEqual(['REGIONS', 'RETURNS']);
  });
});
