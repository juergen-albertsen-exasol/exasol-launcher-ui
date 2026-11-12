import { HttpErrorResponse } from '@angular/common/http';
import { DebugElement } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { Store, provideStore } from '@ngxs/store';
import { firstValueFrom, throwError } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LoadDeploymentsAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { LocalApiFixture, mockLocalApi, runningDetails, summary } from '../../testing/local-api.mock';
import { PageOverviewComponent } from './page-overview.component';

describe('PageOverviewComponent', () => {
  let component: ComponentFixture<PageOverviewComponent>;
  let api: ReturnType<typeof mockLocalApi>;

  const render = async (
    fixture: LocalApiFixture,
    name: string,
    configure: (mock: typeof api) => void = () => undefined
  ): Promise<DebugElement> => {
    api = mockLocalApi(fixture);
    configure(api);
    await TestBed.configureTestingModule({
      imports: [PageOverviewComponent],
      providers: [
        provideNoopAnimations(),
        provideStore([DeploymentsState]),
        { provide: LocalApiService, useValue: api },
      ],
    }).compileComponents();

    await firstValueFrom(TestBed.inject(Store).dispatch(new LoadDeploymentsAction()));

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
});
