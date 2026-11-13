import { Component, inject, input } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { Store } from '@ngxs/store';
import { filter } from 'rxjs';
import { ExaButtonComponent } from '@exasol/ui-common';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import { SidecarDisableDialogComponent } from '@exa/components/sidecar-disable-dialog/sidecar-disable-dialog.component';
import { OperationKind, Sidecar, SidecarHealth } from '@exa/services/local-api/local-api.models';
import { runningLabel } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { endpointText, followSidecars } from '@exa/store/sidecars/sidecar-page';

@Component({
  selector: 'exa-page-sidecars',
  templateUrl: './page-sidecars.component.html',
  styleUrls: ['./page-sidecars.component.scss'],
  imports: [ExaButtonComponent, OperationProgressComponent],
})
export class PageSidecarsComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);

  readonly name = input.required<string>();

  private readonly page = followSidecars(this.name);
  readonly sidecars = this.page.sidecars;
  readonly loadError = this.page.loadError;
  readonly running = this.page.running;
  readonly failed = this.page.failed;
  readonly deploymentStopped = this.page.deploymentStopped;

  readonly health = SidecarHealth;
  readonly runningLabel = runningLabel;
  readonly endpointText = endpointText;

  problems(sidecar: Sidecar): string[] {
    return [sidecar.health_error, sidecar.error, sidecar.last_operation_error].filter((text): text is string => !!text);
  }

  enable(sidecar: Sidecar): void {
    this.store.dispatch(new RunOperationAction(this.name(), OperationKind.ENABLE_SIDECAR, sidecar.name));
  }

  disable(sidecar: Sidecar): void {
    const deployment = this.name();
    SidecarDisableDialogComponent.open(this.dialog, { deployment, sidecar: sidecar.name })
      .pipe(filter(kind => !!kind))
      .subscribe(kind => this.store.dispatch(new RunOperationAction(deployment, kind, sidecar.name)));
  }
}
