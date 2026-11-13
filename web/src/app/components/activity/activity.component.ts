import {
  Component,
  DestroyRef,
  ElementRef,
  HostListener,
  NgZone,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import { Store } from '@ngxs/store';
import { DeploymentPage, deploymentPath } from '@exa/routes';
import { Operation, OperationStatus } from '@exa/services/local-api/local-api.models';
import { formatAgo, formatDuration, operationLabel } from '@exa/store/operations/operation-labels';
import { OperationsState } from '@exa/store/operations/operations.state';

const RECENT_FINISHED = 5;

@Component({
  selector: 'exa-activity',
  templateUrl: './activity.component.html',
  styleUrls: ['./activity.component.scss'],
  imports: [RouterLink],
})
export class ActivityComponent {
  private readonly store = inject(Store);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** `brand` draws the indicator for the start page's green bar. */
  readonly variant = input<'header' | 'brand'>('header');

  readonly running = this.store.selectSignal(OperationsState.running);
  readonly finished = computed(() => this.store.selectSignal(OperationsState.finished)().slice(0, RECENT_FINISHED));
  readonly open = signal(false);
  private readonly now = signal(Date.now());

  readonly progressPath = (operation: Operation): string[] =>
    deploymentPath(operation.deployment, operation.sidecar ? DeploymentPage.SIDECARS : DeploymentPage.OVERVIEW);
  readonly succeeded = OperationStatus.SUCCEEDED;
  readonly label = operationLabel;

  readonly indicatorText = computed(() => {
    const running = this.running();
    return running.length === 1 ? `${operationLabel(running[0])}…` : `${running.length} operations running`;
  });

  constructor() {
    // The clock ticks outside Angular's zone, so pages can still become stable.
    const zone = inject(NgZone);
    const clock = zone.runOutsideAngular(() => setInterval(() => zone.run(() => this.now.set(Date.now())), 1000));
    inject(DestroyRef).onDestroy(() => clearInterval(clock));
  }

  elapsed(operation: Operation): string {
    return formatDuration(this.now() - Date.parse(operation.started_at));
  }

  ago(operation: Operation): string {
    return formatAgo(this.now() - Date.parse(operation.finished_at ?? operation.started_at));
  }

  toggle(): void {
    this.open.update(open => !open);
  }

  close(): void {
    this.open.set(false);
  }

  @HostListener('document:click', ['$event'])
  closeOutside(event: MouseEvent): void {
    if (this.open() && !this.host.nativeElement.contains(event.target as Node)) {
      this.close();
    }
  }

  @HostListener('document:keydown.escape')
  closeOnEscape(): void {
    this.close();
  }
}
