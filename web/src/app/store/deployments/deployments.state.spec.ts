import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Store, provideStore } from '@ngxs/store';
import { NEVER, firstValueFrom, throwError } from 'rxjs';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { LocalApiFixture, mockLocalApi, runningDetails, summary } from '../../testing/local-api.mock';
import { DeploymentRemovedAction, LoadDeploymentsAction, SelectDeploymentAction } from './deployments.actions';
import { DeploymentsState } from './deployments.state';

describe('DeploymentsState', () => {
  let store: Store;
  let api: ReturnType<typeof mockLocalApi>;

  const setUp = (fixture: LocalApiFixture) => {
    api = mockLocalApi(fixture);
    TestBed.configureTestingModule({
      providers: [provideStore([DeploymentsState]), { provide: LocalApiService, useValue: api }],
    });
    store = TestBed.inject(Store);
  };

  it('lists deployments without selecting one', async () => {
    setUp({ deployments: [summary('test'), summary('default')] });

    await firstValueFrom(store.dispatch(new LoadDeploymentsAction()));

    expect(store.selectSnapshot(DeploymentsState.deployments)?.map(d => d.name)).toEqual(['test', 'default']);
    expect(store.selectSnapshot(DeploymentsState.selectedName)).toBeUndefined();
    expect(api.getDeployment).not.toHaveBeenCalled();
  });

  it('loads details and reachability of the selected deployment', async () => {
    setUp({
      deployments: [summary('default'), summary('test', 'stopped')],
      details: { test: { name: 'test', state: 'stopped' } },
      pings: { test: { status: 'unreachable', error: 'Deployment test is stopped' } },
    });
    await firstValueFrom(store.dispatch(new LoadDeploymentsAction()));

    await firstValueFrom(store.dispatch(new SelectDeploymentAction('test')));

    expect(store.selectSnapshot(DeploymentsState.selectedSummary)?.status).toBe('stopped');
    expect(store.selectSnapshot(DeploymentsState.selectedDetails)?.state).toBe('stopped');
    expect(store.selectSnapshot(DeploymentsState.selectedPing)?.status).toBe('unreachable');
  });

  it('reloads the selected deployment when the listing is refreshed', async () => {
    setUp({ deployments: [summary('default')] });
    await firstValueFrom(store.dispatch(new SelectDeploymentAction('default')));

    await firstValueFrom(store.dispatch(new LoadDeploymentsAction(true)));

    expect(api.listDeployments).toHaveBeenLastCalledWith(true);
    expect(api.getDeployment).toHaveBeenCalledTimes(2);
    expect(store.selectSnapshot(DeploymentsState.selectedDetails)).toEqual(runningDetails('default'));
  });

  it('reports an unknown deployment', async () => {
    setUp({ deployments: [] });
    api.getDeployment.mockReturnValue(
      throwError(
        () => new HttpErrorResponse({ status: 404, error: { detail: 'The launcher lists no deployment nope.' } })
      )
    );

    await firstValueFrom(store.dispatch(new SelectDeploymentAction('nope')));

    expect(store.selectSnapshot(DeploymentsState.selectedDetailsError)).toBe('The launcher lists no deployment nope.');
  });

  it('reports a failing listing', async () => {
    setUp({ deployments: [] });
    api.listDeployments.mockReturnValue(
      throwError(() => new HttpErrorResponse({ status: 500, error: { title: 'x', detail: 'launcher failed' } }))
    );

    await firstValueFrom(store.dispatch(new LoadDeploymentsAction()), { defaultValue: undefined });

    expect(store.selectSnapshot(DeploymentsState.listError)).toBe('launcher failed');
  });

  it('drops a removed deployment before the refreshed listing arrives', async () => {
    setUp({ deployments: [summary('default'), summary('test')] });
    await firstValueFrom(store.dispatch(new LoadDeploymentsAction()));
    await firstValueFrom(store.dispatch(new SelectDeploymentAction('test')));
    api.listDeployments.mockReturnValue(NEVER);

    store.dispatch(new DeploymentRemovedAction('test'));

    expect(store.selectSnapshot(DeploymentsState.deployments)?.map(d => d.name)).toEqual(['default']);
    expect(store.selectSnapshot(DeploymentsState.selectedName)).toBeUndefined();
    expect(api.listDeployments).toHaveBeenLastCalledWith(true);
  });
});
