import { HttpErrorResponse } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Actions, Store, ofActionDispatched, provideStore } from '@ngxs/store';
import { firstValueFrom, throwError } from 'rxjs';
import { ErrorNotificationsAction } from '@exasol/ui-common';
import { LocalApiService } from '@exa/services/local-api/local-api.service';
import { mockLocalApi, operation } from '../../testing/local-api.mock';
import { LoadOperationsAction, RunOperationAction } from './operations.actions';
import { OperationsState } from './operations.state';

describe('OperationsState', () => {
  let store: Store;
  let api: ReturnType<typeof mockLocalApi>;

  beforeEach(() => {
    api = mockLocalApi({
      deployments: [],
      operations: [operation({ id: 'a' }), operation({ id: 'b', status: 'succeeded', deployment: 'test' })],
    });
    TestBed.configureTestingModule({
      providers: [provideStore([OperationsState]), { provide: LocalApiService, useValue: api }],
    });
    store = TestBed.inject(Store);
  });

  it('splits loaded operations into running and finished', async () => {
    await firstValueFrom(store.dispatch(new LoadOperationsAction()));

    expect(store.selectSnapshot(OperationsState.running).map(op => op.id)).toEqual(['a']);
    expect(store.selectSnapshot(OperationsState.finished).map(op => op.id)).toEqual(['b']);
  });

  it('adds a started operation at the top', async () => {
    await firstValueFrom(store.dispatch(new LoadOperationsAction()));

    await firstValueFrom(store.dispatch(new RunOperationAction('test', 'stop')));

    expect(api.runOperation).toHaveBeenCalledWith('test', 'stop');
    expect(store.selectSnapshot(OperationsState.operations)[0]).toEqual(
      expect.objectContaining({ id: 'new', deployment: 'test', kind: 'stop' })
    );
  });

  it('reports a refused operation as a notification', async () => {
    const notifications: ErrorNotificationsAction[] = [];
    TestBed.inject(Actions)
      .pipe(ofActionDispatched(ErrorNotificationsAction))
      .subscribe(action => notifications.push(action));
    api.runOperation.mockReturnValue(
      throwError(
        () => new HttpErrorResponse({ status: 409, error: { detail: 'An operation already runs on deployment test.' } })
      )
    );

    await firstValueFrom(store.dispatch(new RunOperationAction('test', 'start')), { defaultValue: undefined });

    expect(notifications.map(action => action.message)).toEqual([
      'Cannot start test: An operation already runs on deployment test.',
    ]);
  });
});
