import { Injectable, inject } from '@angular/core';
import { Store } from '@ngxs/store';
import { Subscription, defer, repeat, timer } from 'rxjs';
import { LoadOperationsAction } from './operations.actions';
import { OPERATIONS_POLL_MS, OperationsState } from './operations.state';

@Injectable({ providedIn: 'root' })
export class OperationsPollingService {
  private readonly store = inject(Store);
  private subscription?: Subscription;

  start(): void {
    this.subscription ??= defer(() => this.store.dispatch(new LoadOperationsAction()))
      .pipe(
        repeat({
          delay: () =>
            timer(
              this.store.selectSnapshot(OperationsState.running).length
                ? OPERATIONS_POLL_MS.running
                : OPERATIONS_POLL_MS.idle
            ),
        })
      )
      .subscribe();
  }

  stop(): void {
    this.subscription?.unsubscribe();
    this.subscription = undefined;
  }
}
