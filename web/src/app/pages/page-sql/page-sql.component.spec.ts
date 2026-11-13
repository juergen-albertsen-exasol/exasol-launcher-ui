import { HttpErrorResponse } from '@angular/common/http';
import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of, throwError } from 'rxjs';
import { SqlEditorComponent } from '@exa/components/sql-editor/sql-editor.component';
import { SchemaSummary, StatementOutcome } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { OperationsState } from '@exa/store/operations/operations.state';
import { SqlState } from '@exa/store/sql/sql.state';
import { LocalApiFixture, mockLocalApi, resultSet, statementFailure, summary } from '../../testing/local-api.mock';
import { PageSqlComponent } from './page-sql.component';

const REGIONS = 'SELECT REGION_NAME FROM REGIONS';

const retailSchemas = (tables = ['REGIONS', 'SALES']): SchemaSummary[] => [
  {
    name: 'RETAIL',
    virtual: false,
    objects: [
      ...tables.map(name => ({ name, kind: 'table' as const })),
      { name: 'SALES_BY_DAY', kind: 'view' as const },
    ],
  },
  { name: 'STAGING', virtual: false, objects: [] },
];

const retailColumns = (): SchemaSummary => ({
  name: 'RETAIL',
  virtual: false,
  objects: [
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
    { name: 'SALES_BY_DAY', kind: 'view', columns: [{ name: 'DAY', type: 'DATE' }] },
  ],
});

const catalogFixture = {
  schemas: { default: retailSchemas() },
  schemaDetails: { default: { RETAIL: retailColumns() } },
};

describe('PageSqlComponent', () => {
  let component: ComponentFixture<PageSqlComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let root: DebugElement;

  const refresh = async () => {
    for (let round = 0; round < 3; round++) {
      component.detectChanges();
      await component.whenStable();
      await new Promise(resolve => setTimeout(resolve));
    }
    component.detectChanges();
  };

  beforeEach(() => localStorage.clear());

  const render = async (fixture: Partial<LocalApiFixture> = {}) => {
    api = mockLocalApi({ deployments: [summary('default'), summary('test')], ...fixture });
    await TestBed.configureTestingModule({
      imports: [PageSqlComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, SqlState]),
        { provide: LocalApiService, useValue: api },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await create();
  };

  const create = async (name = 'default') => {
    component = TestBed.createComponent(PageSqlComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    root = component.debugElement;
  };

  const editor = () => root.query(By.directive(SqlEditorComponent)).componentInstance as SqlEditorComponent;
  const type = async (text: string, cursor = text.length) => {
    editor().replaceAll(text);
    editor().select(cursor);
    await refresh();
  };
  const button = (selector: string) => root.query(By.css(selector)).nativeElement as HTMLButtonElement;
  const click = async (selector: string) => {
    button(selector).click();
    await refresh();
  };
  const text = (selector: string) =>
    (root.query(By.css(selector))?.nativeElement.textContent ?? '').replace(/\s+/g, ' ').trim();
  const tabs = () =>
    root.queryAll(By.css('.sql-results__tab')).map(tab => tab.nativeElement.textContent.replace(/\s+/g, ' ').trim());
  const ran = () => api.runStatement.mock.calls.map(call => call[2]);

  it('runs only the statement at the cursor', async () => {
    await render({
      sqlOutcomes: { [REGIONS]: resultSet([['REGION_NAME', 'VARCHAR(100) UTF8']], [['EMEA']]) },
    });
    const sql = `OPEN SCHEMA RETAIL;\n\n${REGIONS};`;

    await type(sql, sql.indexOf('REGION_NAME'));
    await click('.sql__run');

    expect(ran()).toEqual([REGIONS]);
    expect(tabs()).toEqual(['1SELECT · 1 row']);
  });

  it('runs a selection', async () => {
    await render();
    await type('SELECT 1; SELECT 2; SELECT 3;');
    editor().select(10, 28);

    await click('.sql__run');

    expect(ran()).toEqual(['SELECT 2', 'SELECT 3']);
  });

  it('runs all statements, each with its own tab, and shows the open schema', async () => {
    await render({
      sqlOutcomes: {
        'OPEN SCHEMA RETAIL': { type: 'row_count', row_count: 0, duration_ms: 5, schema: 'RETAIL' },
        [REGIONS]: resultSet(
          [
            ['REGION_NAME', 'VARCHAR(100) UTF8'],
            ['ORDERS', 'DECIMAL(18,0)'],
          ],
          [
            ['Europe', '48213'],
            [null, '112'],
          ],
          { schema: 'RETAIL' }
        ),
      },
    });
    await type(`OPEN SCHEMA RETAIL;\n${REGIONS};`);

    await click('.sql__run-all');

    expect(ran()).toEqual(['OPEN SCHEMA RETAIL', REGIONS]);
    expect(tabs()).toEqual(['1OPEN SCHEMA', '2SELECT · 2 rows']);
    expect(text('.sql__schema')).toBe('Open schema RETAIL');
    expect(text('.sql-results__summary')).toBe('2 rows · 0.08 s');

    const headers = root.queryAll(By.css('.ag-header-cell')).map(cell => cell.nativeElement.textContent.trim());
    expect(headers).toEqual(['#', 'REGION_NAMEVARCHAR(100) UTF8', 'ORDERSDECIMAL(18,0)']);
    const cells = root.queryAll(By.css('.ag-row[row-index="1"] .ag-cell'));
    expect(cells.map(cell => cell.nativeElement.textContent.trim())).toEqual(['2', 'NULL', '112']);
    expect(cells[1].nativeElement.classList).toContain('sql-grid__null');
    expect(cells[2].nativeElement.classList).toContain('ag-right-aligned-cell');

    root.queryAll(By.css('.sql-results__tab'))[0].nativeElement.click();
    await refresh();
    expect(text('.sql-results__message')).toBe('0 rows affected.');
    expect(text('.sql-results__summary')).toBe('0 rows affected · 0.01 s');
  });

  it('stops Run all at a failure and points at the error in the editor', async () => {
    await render({
      sqlFailures: {
        'SELECT r.REGOIN_NAME FROM REGIONS r': statementFailure(
          'object R.REGOIN_NAME not found [line 1, column 10]',
          '42000',
          1,
          10
        ),
      },
    });
    const sql = 'OPEN SCHEMA RETAIL;\n\nSELECT r.REGOIN_NAME FROM REGIONS r;\n\nSELECT 3;';
    await type(sql);

    await click('.sql__run-all');

    expect(tabs()).toEqual(['1OPEN SCHEMA', '2SELECT · failed', '3not run']);
    expect(text('.sql-results__headline')).toBe('The statement failed, so Run all stopped before statement 3.');
    expect(text('.sql-results__error')).toBe('[42000] object R.REGOIN_NAME not found [line 1, column 10]');
    expect(root.query(By.css('.cm-sql-error')).nativeElement.textContent).toBe('r.REGOIN_NAME');
    expect(text('.sql-results__goto')).toBe('Go to line 3, column 10');

    editor().select(0);
    await click('.sql-results__goto');
    expect(editor().cursor()).toBe(sql.indexOf('REGOIN_NAME'));
  });

  it('formats each statement in place', async () => {
    await render();
    await type('-- query\nselect a,b from t where a=1;');

    await click('.sql__format');

    expect(editor().text()).toBe('-- query\nSELECT a,b\nFROM t\nWHERE a=1;');
  });

  it('says when it shows only the first rows and downloads every row', async () => {
    const rows = Array.from({ length: 10_000 }, (_, index) => [String(index)]);
    await render({
      sqlOutcomes: {
        'SELECT N FROM BIG': resultSet([['N', 'DECIMAL(18,0)']], rows, { total_rows: 2_964_624, truncated: true }),
      },
    });
    const clickLink = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    await type('SELECT N FROM BIG');
    await click('.sql__run');

    expect(text('.sql-results__notice')).toBe(
      'The table shows the first 10,000 of 2,964,624 rows. Download CSV saves every row.'
    );
    expect(text('.sql-results__summary')).toBe('First 10,000 of 2,964,624 rows · 0.08 s');

    await click('.sql-results__download');

    expect(api.createExport).toHaveBeenCalledWith('default', 'session-1', 'SELECT N FROM BIG');
    const link = clickLink.mock.contexts[0] as HTMLAnchorElement;
    expect(link.getAttribute('href')).toBe('/api/v1/deployments/default/sql/exports/e1');
    expect(link.download).toBe('default-result.csv');
    clickLink.mockRestore();
  });

  it('shows the running statement and cancels it', async () => {
    await render();
    const slow = new Subject<StatementOutcome>();
    const runDefault = api.runStatement.getMockImplementation()!;
    api.runStatement.mockImplementation((name, id, sql) => (sql === 'SELECT SLOW' ? slow : runDefault(name, id, sql)));
    await type('SELECT 1;\nSELECT SLOW;\nSELECT 3;');

    await click('.sql__run-all');

    expect(text('.sql__progress')).toBe('Running statement 2 of 3 · 0 s');
    expect(root.query(By.css('.sql__run'))).toBeNull();
    expect(root.queryAll(By.css('.cm-sql-running'))).toHaveLength(1);

    await click('.sql__cancel');
    expect(api.cancelStatement).toHaveBeenCalledWith('default', 'session-1');
    slow.error(
      new HttpErrorResponse({ status: 422, error: statementFailure('Client requested execution abort.', 'R0003') })
    );
    await refresh();

    expect(tabs()).toEqual(['1SELECT', '2SELECT · cancelled', '3not run']);
    expect(root.query(By.css('.sql__run'))).toBeTruthy();
  });

  it('says that a new session reset the open schema', async () => {
    await render();
    await type('SELECT 1');
    await click('.sql__run');
    api.runStatement.mockImplementationOnce(() =>
      throwError(() => new HttpErrorResponse({ status: 404, error: { detail: 'the SQL session does not exist' } }))
    );

    await click('.sql__run');

    expect(text('.sql__renewed')).toContain('The SQL session had ended');
    expect(api.openSqlSession).toHaveBeenCalledTimes(2);
  });

  it('waits for a stopped database and starts it', async () => {
    await render({ deployments: [summary('default', 'stopped')] });

    expect(text('.sql__banner')).toContain('The database is not running.');
    expect(button('.sql__run').disabled).toBe(true);
    expect(button('.sql__run-all').disabled).toBe(true);
    expect(button('.sql__format').disabled).toBe(false);
    expect(text('.sql-results__empty')).toBe(
      'Your SQL stays here while the database is stopped. Results appear once it runs.'
    );

    (root.query(By.css('.sql__start button')) ?? root.query(By.css('.sql__start'))).nativeElement.click();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'start');
  });

  it('closes its session when the page goes away', async () => {
    await render();
    await type('SELECT 1');
    await click('.sql__run');

    component.destroy();

    expect(api.closeSqlSession).toHaveBeenCalledWith('default', 'session-1');
  });

  const nodes = (selector = '.schemas__node') =>
    root.queryAll(By.css(selector)).map(node => node.nativeElement.textContent.replace(/\s+/g, ' ').trim());
  const node = (label: string) =>
    root
      .queryAll(By.css('.schemas__node'))
      .find(element => element.nativeElement.textContent.replace(/\s+/g, ' ').trim().startsWith(label))!
      .nativeElement as HTMLButtonElement;

  it('lists the schemas and expands them to tables and columns', async () => {
    await render(catalogFixture);

    expect(nodes()).toEqual(['RETAIL', 'STAGING']);

    node('RETAIL').click();
    await refresh();
    expect(nodes('.schemas__table')).toEqual(['REGIONS', 'SALES', 'SALES_BY_DAY']);
    expect(root.query(By.css('.schemas__table .fa-eye'))).toBeTruthy();

    node('REGIONS').click();
    await refresh();
    expect(api.getSchema).toHaveBeenCalledWith('default', 'RETAIL');
    expect(nodes('.schemas__column')).toEqual(['REGION_IDDECIMAL(9,0)', 'REGION_NAMEVARCHAR(100) UTF8']);
  });

  it('filters schemas, tables, and columns by name', async () => {
    await render(catalogFixture);

    const filter = root.query(By.css('.schemas__search input')).nativeElement as HTMLInputElement;
    filter.value = 'region';
    filter.dispatchEvent(new Event('input'));
    await refresh();

    expect(nodes()).toEqual([
      'RETAIL',
      'REGIONS',
      'REGION_IDDECIMAL(9,0)',
      'REGION_NAMEVARCHAR(100) UTF8',
      'SALES',
      'REGION_IDDECIMAL(9,0)',
    ]);
  });

  it('inserts a double-clicked name at the cursor and completes names in the editor', async () => {
    await render(catalogFixture);
    await type('SELECT  FROM T', 7);

    node('RETAIL').click();
    await refresh();
    node('SALES_BY_DAY').dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    await refresh();

    expect(editor().text()).toBe('SELECT SALES_BY_DAY FROM T');
    expect(
      editor()
        .catalog()
        ?.schemas.map(schema => schema.name)
    ).toEqual(['RETAIL', 'STAGING']);
  });

  it('shows a new table without a reload', async () => {
    await render(catalogFixture);
    api.listSchemas.mockReturnValue(of(retailSchemas(['REGIONS', 'RETURNS', 'SALES'])));
    await type('CREATE TABLE RETAIL.RETURNS (ID INT)');

    await click('.sql__run');
    node('RETAIL').click();
    await refresh();

    expect(nodes('.schemas__table')).toEqual(['REGIONS', 'RETURNS', 'SALES', 'SALES_BY_DAY']);
  });

  it('reloads the schemas', async () => {
    await render(catalogFixture);

    await click('.schemas__reload');

    expect(api.listSchemas).toHaveBeenCalledTimes(2);
  });

  it('lists the schemas once the database runs', async () => {
    await render({ ...catalogFixture, deployments: [summary('default', 'stopped')] });

    expect(text('.schemas__hint')).toBe('Available once the database runs.');
    expect(root.query(By.css('.schemas__search'))).toBeNull();
    expect(api.listSchemas).not.toHaveBeenCalled();
  });

  it('keeps the text of each deployment', async () => {
    await render();
    await type('SELECT 1 -- default');

    component.destroy();
    await create();
    expect(editor().text()).toBe('SELECT 1 -- default');

    component.componentRef.setInput('name', 'test');
    await refresh();
    expect(editor().text()).toBe('');

    await type('SELECT 2 -- test');
    component.componentRef.setInput('name', 'default');
    await refresh();
    expect(editor().text()).toBe('SELECT 1 -- default');
  });

  it('shows the saved text while the database is stopped', async () => {
    localStorage.setItem('exasol-launcher-ui:sql:default:text', 'SELECT 42');

    await render({ deployments: [summary('default', 'stopped')] });

    expect(editor().text()).toBe('SELECT 42');
  });

  it('lists, searches, inserts, and clears the history', async () => {
    await render({
      sqlOutcomes: { 'SELECT 1': resultSet([['1', 'DECIMAL(1,0)']], [['1']]) },
      sqlFailures: { 'SELECT X': statementFailure('object X not found [line 1, column 8]') },
    });
    await type('SELECT 1;\nSELECT X;');
    await click('.sql__run-all');

    await click('.history__toggle');
    const entries = () =>
      root.queryAll(By.css('.history__entry')).map(entry => ({
        sql: entry.query(By.css('.history__sql')).nativeElement.textContent.trim(),
        when: entry.query(By.css('.history__when')).nativeElement.textContent.trim(),
        sub: entry.query(By.css('.history__sub')).nativeElement.textContent.trim(),
      }));
    expect(entries()).toEqual([
      { sql: 'SELECT X', when: 'just now', sub: '[42000] object X not found [line 1, column 8]' },
      { sql: 'SELECT 1', when: 'just now', sub: '1 row · 0.08 s' },
    ]);
    expect(root.queryAll(By.css('.history__entry .history__bad'))).toHaveLength(1);

    const search = root.query(By.css('.history__search input')).nativeElement as HTMLInputElement;
    search.value = 'x';
    search.dispatchEvent(new Event('input'));
    await refresh();
    expect(entries().map(entry => entry.sql)).toEqual(['SELECT X']);

    editor().select(0);
    await click('.history__entry');
    expect(editor().text()).toBe('SELECT XSELECT 1;\nSELECT X;');
    expect(root.query(By.css('.history__menu'))).toBeNull();

    await click('.history__toggle');
    await click('.history__clear');
    expect(text('.history__empty')).toBe('No statements have run on this deployment in this browser.');
    expect(localStorage.getItem('exasol-launcher-ui:sql:default:history')).toBeNull();
  });
});
