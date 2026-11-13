import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Actions, Store, ofActionDispatched, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of } from 'rxjs';
import { ErrorNotificationsAction, SuccessNotificationsAction } from '@exasol/ui-common';
import { DeploymentSummary, Operation } from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEvent, OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { DiagnosticsState } from '@exa/store/diagnostics/diagnostics.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import {
  LocalApiFixture,
  diagnosticReport,
  mockLocalApi,
  operation,
  runningLocalRuntime,
  stoppedLocalRuntime,
  summary,
} from '../../testing/local-api.mock';
import { PageDiagnosticsComponent, shellArgument } from './page-diagnostics.component';

const notificationsOf = (actions: Actions): unknown[] => {
  const dispatched: unknown[] = [];
  actions
    .pipe(ofActionDispatched(SuccessNotificationsAction, ErrorNotificationsAction))
    .subscribe(action => dispatched.push(action));
  return dispatched;
};

const readText = (blob: Blob): Promise<unknown> =>
  new Promise(resolve => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.readAsText(blob);
  });

const cloud = (name: string): DeploymentSummary => ({ ...summary(name, 'stopped'), infrastructure: 'aws' });

describe('PageDiagnosticsComponent', () => {
  let component: ComponentFixture<PageDiagnosticsComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let events: Subject<OperationEvent>;
  let dialogResult: unknown;
  let writeText: jest.Mock;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: LocalApiFixture, name = 'default'): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    events = new Subject<OperationEvent>();
    dialogResult = undefined;
    writeText = jest.fn(() => Promise.resolve());
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    await TestBed.configureTestingModule({
      imports: [PageDiagnosticsComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, DiagnosticsState]),
        { provide: LocalApiService, useValue: api },
        { provide: MatDialog, useValue: { open: jest.fn(() => ({ afterClosed: () => of(dialogResult) })) } },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => events) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    component = TestBed.createComponent(PageDiagnosticsComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    return component.debugElement;
  };

  const text = (element: DebugElement, selector: string) =>
    (element.query(By.css(selector)).nativeElement.textContent as string).replace(/\s+/g, ' ').trim();
  const button = (root: DebugElement, selector: string) =>
    root.query(By.css(selector)).nativeElement as HTMLButtonElement;
  const ports = (root: DebugElement) =>
    root.queryAll(By.css('.diagnostics__port')).map(row => ({
      name: text(row, '.diagnostics__port-name'),
      cells: row.queryAll(By.css('td')).map(cell => (cell.nativeElement.textContent as string).trim()),
      reachable: row.query(By.css('.diagnostics__reachability')).classes['diagnostics__word--ok'] ?? false,
    }));

  it('shows the runtime of a running local deployment', async () => {
    const root = await render({ deployments: [summary('default')] });

    expect(api.getLocalRuntime).toHaveBeenCalledWith('default');
    expect(text(root, '.diagnostics__platform')).toBe('darwin/arm64, supported');
    expect(text(root, '.diagnostics__vm')).toBe('running');
    expect(text(root, '.diagnostics__database')).toBe('ready');
    expect(root.queryAll(By.css('.diagnostics__table th')).map(cell => cell.nativeElement.textContent.trim())).toEqual([
      'Port',
      'Host port',
      'Reachability',
    ]);
    expect(ports(root)).toEqual([
      { name: 'db', cells: ['db', '8563', 'reachable'], reachable: true },
      { name: 'sidecar mcp', cells: ['sidecar mcp', '4896', 'reachable'], reachable: true },
    ]);
  });

  it('shows a database that is not ready with its error', async () => {
    const runtime = { ...runningLocalRuntime(), database_ready: false, database_error: 'connection refused' };
    const root = await render({ deployments: [summary('default')], localRuntimes: { default: runtime } });

    expect(text(root, '.diagnostics__database')).toBe('not ready connection refused');
  });

  it('checks again and shows the new reachability', async () => {
    const root = await render({ deployments: [summary('default')] });
    const failing = runningLocalRuntime();
    failing.ports[1].reachability = 'refused';
    const check = new Subject<typeof failing>();
    api.getLocalRuntime.mockReturnValue(check);

    button(root, '.diagnostics__check').click();
    await refresh();

    expect(button(root, '.diagnostics__check').disabled).toBe(true);
    expect(text(root, '.diagnostics__check')).toBe('Checking…');

    check.next(failing);
    check.complete();
    await refresh();

    expect(ports(root)[1]).toEqual({
      name: 'sidecar mcp',
      cells: ['sidecar mcp', '4896', 'refused'],
      reachable: false,
    });
    expect(button(root, '.diagnostics__check').disabled).toBe(false);
  });

  it("shows a stopped deployment's VM and the launcher's message", async () => {
    const root = await render({
      deployments: [summary('default', 'stopped')],
      localRuntimes: { default: stoppedLocalRuntime() },
    });

    expect(text(root, '.diagnostics__vm')).toBe('stopped');
    expect(root.query(By.css('.diagnostics__database'))).toBeNull();
    expect(text(root, '.diagnostics__note')).toContain('Run `exasol start` to start it');
    expect(root.query(By.css('.diagnostics__table'))).toBeNull();
  });

  it('shows the launcher warning', async () => {
    const runtime = { ...runningLocalRuntime(), warning: 'the VM runs without a recorded workflow' };
    const root = await render({ deployments: [summary('default')], localRuntimes: { default: runtime } });

    expect(text(root, '.diagnostics__warning')).toBe('the VM runs without a recorded workflow');
  });

  it('shows why the local runtime cannot be checked', async () => {
    const root = await render({
      deployments: [summary('default')],
      localRuntimeRefusals: { default: 'another operation is in progress' },
    });

    expect(text(root, '.diagnostics__local .diagnostics__message--error')).toBe('another operation is in progress');
  });

  it('offers no local runtime for a cloud deployment', async () => {
    const root = await render({ deployments: [cloud('aws')] }, 'aws');

    expect(root.query(By.css('.diagnostics__local'))).toBeNull();
    expect(api.getLocalRuntime).not.toHaveBeenCalled();
    expect(root.query(By.css('.diagnostics__info'))).toBeTruthy();
  });

  it('shows and hides the report as formatted JSON', async () => {
    const root = await render({ deployments: [summary('default')] });
    expect(api.getDiagnosticReport).not.toHaveBeenCalled();

    button(root, '.diagnostics__show').click();
    await refresh();

    expect(api.getDiagnosticReport).toHaveBeenCalledWith('default');
    expect(root.query(By.css('.diagnostics__report')).nativeElement.textContent).toBe(
      JSON.stringify(diagnosticReport(), null, 2)
    );
    expect(text(root, '.diagnostics__show')).toBe('Hide');

    button(root, '.diagnostics__show').click();
    await refresh();

    expect(root.query(By.css('.diagnostics__report'))).toBeNull();
    expect(text(root, '.diagnostics__show')).toBe('Show');
  });

  describe('downloading the report', () => {
    const createObjectURL = jest.fn((blob: Blob) => (blob ? 'blob:report' : ''));
    const revokeObjectURL = jest.fn();
    let saved: { download: string; href: string }[];

    beforeEach(() => {
      Object.assign(URL, { createObjectURL, revokeObjectURL });
      saved = [];
      jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
        saved.push({ download: this.download, href: this.href });
      });
    });

    afterEach(() => jest.restoreAllMocks());

    it('saves the report as a file named after the deployment', async () => {
      const root = await render({ deployments: [summary('default')] });

      button(root, '.diagnostics__download').click();
      await refresh();

      expect(api.getDiagnosticReport).toHaveBeenCalledWith('default');
      expect(saved).toEqual([{ download: 'default-info.json', href: 'blob:report' }]);
      const blob = createObjectURL.mock.calls[0][0];
      expect(await readText(blob)).toBe(JSON.stringify(diagnosticReport(), null, 2));
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:report');
    });
  });

  it('shows a running operation, keeps Check again unavailable, and checks again when it ends', async () => {
    const root = await render({
      deployments: [summary('default')],
      operations: [operation({ id: 'op-1', kind: 'start', status: 'running' })],
    });
    api.getLocalRuntime.mockClear();

    expect(text(root, '.operation-progress__text')).toContain('Starting default.');
    expect(button(root, '.diagnostics__check').disabled).toBe(true);

    const finished: Operation = operation({ id: 'op-1', kind: 'start', status: 'succeeded' });
    api.listOperations.mockReturnValue(of([finished]));
    events.next({ type: 'status', operation: finished });
    await refresh();

    expect(root.query(By.css('.operation-progress'))).toBeNull();
    expect(api.getLocalRuntime).toHaveBeenCalledWith('default');
    expect(button(root, '.diagnostics__check').disabled).toBe(false);
  });

  it('shows the shell commands and copies them', async () => {
    const root = await render({ deployments: [summary('default')] });

    const shell = root.query(By.css('.diagnostics__shell'));
    expect(shell.queryAll(By.css('.diagnostics__lead')).map(lead => lead.nativeElement.textContent.trim())).toEqual([
      'Open a shell on the host node:',
      'Run one command on the host node:',
      'Open a shell in the database container:',
    ]);
    expect(shell.queryAll(By.css('.diagnostics__command')).map(command => command.nativeElement.textContent)).toEqual([
      'exasol shell host -d default',
      'exasol shell host -d default -- [command]',
      'exasol shell container -d default',
    ]);

    shell.queryAll(By.css('button[aria-label="Copy the command"]'))[2].nativeElement.click();

    expect(writeText).toHaveBeenCalledWith('exasol shell container -d default');
  });

  it('quotes deployment names for the shell', () => {
    expect(shellArgument('my-test_1.2')).toBe('my-test_1.2');
    expect(shellArgument('my test')).toBe("'my test'");
    expect(shellArgument("it's")).toBe("'it'\\''s'");
  });

  it('unlocks the folder after confirmation and reports it', async () => {
    const root = await render({ deployments: [summary('default')] });
    const notifications = notificationsOf(TestBed.inject(Actions));
    dialogResult = true;

    button(root, '.diagnostics__unlock').click();
    await refresh();

    expect(TestBed.inject(MatDialog).open).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        data: expect.objectContaining({
          title: 'Unlock default?',
          content: expect.stringContaining('can leave its files inconsistent'),
        }),
      })
    );
    expect(api.unlock).toHaveBeenCalledWith('default');
    expect(notifications).toContainEqual(new SuccessNotificationsAction('Unlocked the folder of default.'));
  });

  it('leaves the folder locked when the confirmation is cancelled', async () => {
    const root = await render({ deployments: [summary('default')] });

    button(root, '.diagnostics__unlock').click();
    await refresh();

    expect(api.unlock).not.toHaveBeenCalled();
  });

  it('reports a refused unlock', async () => {
    const root = await render({
      deployments: [summary('default')],
      unlockRefusals: { default: 'An operation runs on deployment default, so its folder stays locked.' },
    });
    const notifications = notificationsOf(TestBed.inject(Actions));
    dialogResult = true;

    button(root, '.diagnostics__unlock').click();
    await refresh();

    expect(notifications).toContainEqual(
      new ErrorNotificationsAction(
        'Cannot unlock the folder of default: An operation runs on deployment default, so its folder stays locked.'
      )
    );
  });

  it('keeps Unlock unavailable while an operation runs', async () => {
    const root = await render({
      deployments: [summary('default')],
      operations: [operation({ id: 'op-1', kind: 'start', status: 'running' })],
    });

    expect(root.query(By.css('.operation-progress'))).toBeTruthy();
    expect(button(root, '.diagnostics__unlock').disabled).toBe(true);
  });
});
