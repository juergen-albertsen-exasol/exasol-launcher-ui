import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { provideRouter } from '@angular/router';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEvent, OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { STARTING_REFRESH_MS } from '@exa/store/sidecars/sidecar-page';
import { SidecarsState } from '@exa/store/sidecars/sidecars.state';
import {
  LocalApiFixture,
  defaultSidecars,
  mockLocalApi,
  operation,
  sidecar,
  summary,
} from '../../testing/local-api.mock';
import { PageSidecarsComponent } from './page-sidecars.component';

describe('PageSidecarsComponent', () => {
  let component: ComponentFixture<PageSidecarsComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let dialogResult: unknown;
  let events: Subject<OperationEvent>;

  const refresh = async () => {
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
  };

  const render = async (fixture: LocalApiFixture, name = 'default'): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    dialogResult = undefined;
    events = new Subject<OperationEvent>();
    await TestBed.configureTestingModule({
      imports: [PageSidecarsComponent],
      providers: [
        provideNoopAnimations(),
        provideRouter([]),
        provideStore([DeploymentsState, OperationsState, SidecarsState]),
        { provide: LocalApiService, useValue: api },
        { provide: MatDialog, useValue: { open: jest.fn(() => ({ afterClosed: () => of(dialogResult) })) } },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => events) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));
    component = TestBed.createComponent(PageSidecarsComponent);
    component.componentRef.setInput('name', name);
    await refresh();
    return component.debugElement;
  };

  const rows = (root: DebugElement) => root.queryAll(By.css('.sidecars__row'));
  const row = (root: DebugElement, name: string) =>
    rows(root).find(element => element.query(By.css('.sidecars__name')).nativeElement.textContent.trim() === name)!;
  const text = (element: DebugElement, selector: string) =>
    (element.query(By.css(selector)).nativeElement.textContent as string).replace(/\s+/g, ' ').trim();
  const click = (element: DebugElement, selector: string) => {
    const target = element.query(By.css(selector)).nativeElement as HTMLElement;
    (target.querySelector('button') ?? target).click();
  };

  it('lists the sidecars with their state, health, and endpoint', async () => {
    const root = await render({ deployments: [summary('default')] });

    const headings = root.queryAll(By.css('.sidecars__table th')).map(cell => cell.nativeElement.textContent.trim());
    expect(headings).toEqual(['Sidecar', 'Description', 'State', 'Health', 'Endpoint', 'Action']);
    expect(root.query(By.css('.sidecars__box')).attributes['data-label']).toBe('Sidecars of default');
    expect(rows(root).map(element => text(element, '.sidecars__name'))).toEqual(['mcp', 'kafka', 'dash-server']);

    const mcp = row(root, 'mcp');
    expect(text(mcp, '.sidecars__description')).toBe('Exasol MCP server with read-oriented query tools');
    expect(text(mcp, '.sidecars__state')).toBe('running');
    expect(text(mcp, '.sidecars__health')).toBe('healthy');
    const link = mcp.query(By.css('.sidecars__link')).nativeElement as HTMLAnchorElement;
    expect(link.textContent?.trim()).toBe('127.0.0.1:4896/mcp');
    expect(link.href).toBe('http://127.0.0.1:4896/mcp');

    const kafka = row(root, 'kafka');
    expect(text(kafka, '.sidecars__endpoints')).toBe('127.0.0.1:19092');
    expect(kafka.query(By.css('.sidecars__link'))).toBeNull();
    expect(text(kafka, '.sidecars__health')).toBe('');
    expect(kafka.query(By.css('.sidecars__disable'))).toBeTruthy();

    const dashServer = row(root, 'dash-server');
    expect(text(dashServer, '.sidecars__state')).toBe('disabled');
    expect(dashServer.query(By.css('.sidecars__enable'))).toBeTruthy();
    expect(dashServer.query(By.css('.sidecars__disable'))).toBeNull();
    expect(root.query(By.css('.sidecars__banner'))).toBeNull();
  });

  it('shows health errors and operation errors below their sidecar', async () => {
    const root = await render({
      deployments: [summary('default')],
      sidecars: {
        default: [
          sidecar({
            health: 'failing',
            health_error: 'Get "http://127.0.0.1:4896/health": context deadline exceeded',
            last_operation_error: 'enable timed out',
          }),
        ],
      },
    });

    expect(text(row(root, 'mcp'), '.sidecars__health')).toBe('failing');
    const problems = root
      .queryAll(By.css('.sidecars__problem'))
      .map(problem => problem.nativeElement.textContent.trim());
    expect(problems).toEqual(['Get "http://127.0.0.1:4896/health": context deadline exceeded', 'enable timed out']);
  });

  it('enables a sidecar and lists it as enabled once the operation succeeds', async () => {
    const root = await render({ deployments: [summary('default')] });

    click(row(root, 'dash-server'), '.sidecars__enable');
    await refresh();

    expect(api.runOperation).toHaveBeenCalledWith('default', 'enable_sidecar', 'dash-server');
    events.next({ type: 'line', line: 'Pulling the image' });
    await refresh();
    expect(text(root, '.operation-progress__text')).toBe('Enabling dash-server on default.');
    expect(text(root, '.operation-progress__log')).toBe('Pulling the image');
    expect((row(root, 'mcp').query(By.css('.sidecars__disable')).nativeElement as HTMLButtonElement).disabled).toBe(
      true
    );

    const enabled = defaultSidecars().map(entry =>
      entry.name === 'dash-server' ? { ...entry, enabled: true, running: true, state: 'running' } : entry
    );
    api.listSidecars.mockReturnValue(of(enabled));
    const finished = operation({ id: 'new', kind: 'enable_sidecar', sidecar: 'dash-server', status: 'succeeded' });
    api.listOperations.mockReturnValue(of([finished]));
    events.next({ type: 'status', operation: finished });
    await refresh();

    expect(root.query(By.css('.operation-progress'))).toBeNull();
    expect(text(row(root, 'dash-server'), '.sidecars__state')).toBe('running');
    expect(row(root, 'dash-server').query(By.css('.sidecars__disable'))).toBeTruthy();
  });

  it('disables a sidecar and deletes its definition when the confirmation asks for it', async () => {
    const root = await render({ deployments: [summary('default')] });
    dialogResult = 'remove_sidecar';

    click(row(root, 'kafka'), '.sidecars__disable');
    await refresh();

    expect(TestBed.inject(MatDialog).open).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        data: { deployment: 'default', sidecar: 'kafka' },
      })
    );
    expect(api.runOperation).toHaveBeenCalledWith('default', 'remove_sidecar', 'kafka');

    api.listSidecars.mockReturnValue(of(defaultSidecars().filter(entry => entry.name !== 'kafka')));
    const finished = operation({ id: 'new', kind: 'remove_sidecar', sidecar: 'kafka', status: 'succeeded' });
    api.listOperations.mockReturnValue(of([finished]));
    events.next({ type: 'status', operation: finished });
    await refresh();

    expect(rows(root).map(element => text(element, '.sidecars__name'))).toEqual(['mcp', 'dash-server']);
  });

  it('runs nothing when the user cancels the confirmation', async () => {
    const root = await render({ deployments: [summary('default')] });

    click(row(root, 'kafka'), '.sidecars__disable');
    await refresh();

    expect(api.runOperation).not.toHaveBeenCalled();
  });

  it('says that changes take effect when a stopped deployment starts', async () => {
    const root = await render({ deployments: [summary('default', 'stopped')] });

    expect(text(root, '.sidecars__banner')).toBe(
      'The deployment is not running. Sidecar changes take effect when it starts.'
    );
  });

  it('shows the launcher message for deployments without sidecars', async () => {
    const root = await render(
      {
        deployments: [summary('aws')],
        sidecarRefusals: { aws: 'sidecars are not supported for this deployment type' },
      },
      'aws'
    );

    expect(text(root, '.sidecars__message--error')).toBe('sidecars are not supported for this deployment type');
    expect(root.query(By.css('.sidecars__table'))).toBeNull();
  });

  it('reports a failed operation above the current sidecars', async () => {
    const root = await render({
      deployments: [summary('default')],
      operations: [
        operation({
          kind: 'enable_sidecar',
          sidecar: 'dash-server',
          status: 'failed',
          error: 'image pull failed',
        }),
      ],
    });

    expect(text(root, '.sidecars__failure')).toBe('Enabling dash-server on default failed: image pull failed');
    expect(rows(root)).toHaveLength(3);
  });

  describe('while a sidecar starts', () => {
    afterEach(() => jest.useRealTimers());

    it('refreshes the sidecars until the sidecar serves', async () => {
      jest.useFakeTimers({ doNotFake: ['nextTick', 'setImmediate', 'queueMicrotask'] });
      const root = await render({
        deployments: [summary('default')],
        sidecars: { default: [sidecar({ health: 'starting' })] },
      });
      expect(text(row(root, 'mcp'), '.sidecars__health')).toBe('starting');
      api.listSidecars.mockClear();
      api.listSidecars.mockReturnValue(of([sidecar()]));

      jest.advanceTimersByTime(STARTING_REFRESH_MS);
      await refresh();

      expect(api.listSidecars).toHaveBeenCalledTimes(1);
      expect(text(row(root, 'mcp'), '.sidecars__health')).toBe('healthy');

      jest.advanceTimersByTime(STARTING_REFRESH_MS * 2);
      expect(api.listSidecars).toHaveBeenCalledTimes(1);
    });
  });
});
