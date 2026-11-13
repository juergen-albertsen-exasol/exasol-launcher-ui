import { HttpErrorResponse } from '@angular/common/http';
import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Store, provideStore } from '@ngxs/store';
import { Subject, firstValueFrom, of, throwError } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { OperationEvent, OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';
import { LocalApiFixture, mockLocalApi, operation, runningDetails, summary } from '../../testing/local-api.mock';
import { PageOverviewComponent } from './page-overview.component';

describe('PageOverviewComponent', () => {
  let component: ComponentFixture<PageOverviewComponent>;
  let api: ReturnType<typeof mockLocalApi>;
  let events: Subject<OperationEvent>;

  const render = async (
    fixture: LocalApiFixture,
    name: string,
    configure: (mock: typeof api) => void = () => undefined
  ): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    configure(api);
    events = new Subject<OperationEvent>();
    await TestBed.configureTestingModule({
      imports: [PageOverviewComponent],
      providers: [
        provideNoopAnimations(),
        provideStore([DeploymentsState, OperationsState]),
        { provide: LocalApiService, useValue: api },
        { provide: OperationEventsService, useValue: { follow: jest.fn(() => events) } },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadOperationsAction()));

    component = TestBed.createComponent(PageOverviewComponent);
    component.componentRef.setInput('name', name);
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();
    return component.debugElement;
  };

  const text = (root: DebugElement, selector: string) => root.query(By.css(selector)).nativeElement.textContent.trim();

  it('shows the name, state, status, folder, presets, connection details, and reachability of its deployment', async () => {
    const root = await render({ deployments: [summary('default')] }, 'default');

    expect(text(root, '.overview__name')).toBe('default');
    expect(text(root, '.overview__state')).toBe('running');
    expect(text(root, '.overview__host')).toBe('127.0.0.1');
    expect(text(root, '.overview__port')).toBe('8563');
    expect(text(root, '.overview__username')).toBe('sys');
    expect(text(root, '.overview__path')).toBe('/home/user/.exasol/personal/deployments/default');
    expect(text(root, '.overview__status')).toBe('database_ready');
    expect(text(root, '.overview__infrastructure')).toBe('local');
    expect(text(root, '.overview__state-word--ok')).toContain('reachable');
    expect(text(root, '.overview__reachability')).toContain('5 ms');
  });

  it('shows the error of an unreachable database', async () => {
    const root = await render(
      {
        deployments: [summary('default')],
        pings: { default: { status: 'unreachable', error: 'cannot query the database as sys@127.0.0.1:8563' } },
      },
      'default'
    );

    expect(text(root, '.overview__state-word--error')).toContain('unreachable');
    expect(text(root, '.overview__error')).toContain('sys@127.0.0.1:8563');
  });

  it('switches to another deployment', async () => {
    const root = await render(
      { deployments: [summary('default'), summary('test')], details: { test: runningDetails('test', 8564) } },
      'default'
    );

    component.componentRef.setInput('name', 'test');
    component.detectChanges();
    await component.whenStable();
    component.detectChanges();

    expect(text(root, '.overview__name')).toBe('test');
    expect(text(root, '.overview__port')).toBe('8564');
  });

  it('reports an unknown deployment', async () => {
    const root = await render({ deployments: [summary('default')] }, 'nope', mock =>
      mock.getDeployment.mockReturnValue(
        throwError(
          () => new HttpErrorResponse({ status: 404, error: { detail: 'The launcher lists no deployment nope.' } })
        )
      )
    );

    expect(text(root, '.overview__message--error')).toContain('lists no deployment nope');
  });

  describe('start and stop', () => {
    const stoppedDetails = { default: { name: 'default', state: 'stopped' } };
    const refresh = async () => {
      component.detectChanges();
      await component.whenStable();
      component.detectChanges();
    };

    it('offers Start for a stopped deployment and starts it', async () => {
      const root = await render({ deployments: [summary('default', 'stopped')], details: stoppedDetails }, 'default');

      expect(root.query(By.css('.overview__stop'))).toBeNull();
      root.query(By.css('.overview__start button')).nativeElement.click();

      expect(api.runOperation).toHaveBeenCalledWith('default', 'start');
    });

    it('offers Stop beside Check again for a running deployment and stops it', async () => {
      const root = await render({ deployments: [summary('default')] }, 'default');

      expect(root.query(By.css('.overview__start'))).toBeNull();
      root.query(By.css('.overview__stop')).nativeElement.click();

      expect(api.runOperation).toHaveBeenCalledWith('default', 'stop');
    });

    it('shows the progress output in place of the details while an operation runs', async () => {
      const root = await render(
        { deployments: [summary('default', 'stopped')], details: stoppedDetails, operations: [operation()] },
        'default'
      );

      events.next({ type: 'line', line: 'Starting the VM' });
      events.next({ type: 'line', line: 'Waiting for the database' });
      await refresh();

      expect(text(root, '.overview__state')).toBe('starting');
      expect(text(root, '.operation-progress__log')).toBe('Starting the VM\nWaiting for the database');
      expect(root.query(By.css('.overview__box[data-label="Connection"]'))).toBeNull();
      expect(root.query(By.css('.overview__start'))).toBeNull();
    });

    it('shows the new state when the operation ends', async () => {
      await render(
        { deployments: [summary('default', 'stopped')], details: stoppedDetails, operations: [operation()] },
        'default'
      );
      api.listOperations.mockReturnValue(of([operation({ status: 'succeeded' })]));
      api.getDeployment.mockReturnValue(of(runningDetails('default')));
      api.listDeployments.mockReturnValue(of([summary('default')]));

      events.next({ type: 'status', operation: operation({ status: 'succeeded' }) });
      await refresh();

      expect(api.listDeployments).toHaveBeenLastCalledWith(true);
      expect(text(component.debugElement, '.overview__state')).toBe('running');
      expect(component.debugElement.query(By.css('.overview__stop'))).toBeTruthy();
    });

    it('reports a failed operation', async () => {
      const root = await render(
        {
          deployments: [summary('default')],
          operations: [operation({ kind: 'stop', status: 'failed', error: 'Error: the VM did not stop' })],
        },
        'default'
      );

      expect(text(root, '.overview__failure')).toContain('Stopping default failed: Error: the VM did not stop');
    });
  });
});
