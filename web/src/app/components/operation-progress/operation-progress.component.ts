import { Component, ElementRef, computed, effect, inject, input, signal, untracked, viewChild } from '@angular/core';
import { Store } from '@ngxs/store';
import { Operation, OperationKind } from '@exa/services/local-api/local-api.models';
import { OperationEventsService } from '@exa/services/local-api/operation-events.service';
import { RUNNING_VERB, runningLabel } from '@exa/store/operations/operation-labels';
import { LoadOperationsAction } from '@exa/store/operations/operations.actions';

/** Shows a running operation with the launcher output it streams. */
@Component({
  selector: 'exa-operation-progress',
  templateUrl: './operation-progress.component.html',
  styleUrls: ['./operation-progress.component.scss'],
})
export class OperationProgressComponent {
  private readonly store = inject(Store);
  private readonly events = inject(OperationEventsService);
  private readonly log = viewChild<ElementRef<HTMLElement>>('log');

  readonly operation = input.required<Operation>();

  readonly operationKind = OperationKind;
  readonly title = computed(() => RUNNING_VERB[this.operation().kind]);
  readonly label = computed(() => runningLabel(this.operation()));
  readonly lines = signal<string[]>([]);

  constructor() {
    effect(onCleanup => {
      const id = this.operation().id;
      untracked(() => this.lines.set([]));
      const subscription = this.events.follow(id).subscribe(event => {
        if (event.type === 'line') {
          this.lines.update(lines => [...lines, event.line]);
        } else {
          this.store.dispatch(new LoadOperationsAction());
        }
      });
      onCleanup(() => subscription.unsubscribe());
    });
    effect(() => {
      this.lines();
      const log = this.log()?.nativeElement;
      if (log) {
        log.scrollTop = log.scrollHeight;
      }
    });
  }
}
