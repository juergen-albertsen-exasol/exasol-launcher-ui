import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { Router } from '@angular/router';
import { Store } from '@ngxs/store';
import { ExaButtonComponent } from '@exasol/ui-common';
import { DeploymentPage, deploymentPath } from '@exa/routes';
import {
  DEPLOYMENT_READY_STATUS,
  DataImport,
  ImportStatus,
  Operation,
  OperationKind,
  OperationStatus,
} from '@exa/services/local-api/local-api.models';
import { appendText } from '@exa/sql/storage';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import { formatCount, formatSeconds, queryStatement } from '@exa/store/imports/import-labels';
import { LoadImportsAction, UploadFilesAction } from '@exa/store/imports/imports.actions';
import { ImportsState, emptyImportPage } from '@exa/store/imports/imports.state';
import { RUNNING_VERB } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { OperationsState } from '@exa/store/operations/operations.state';

const PERCENT = 100;

@Component({
  selector: 'exa-page-import',
  templateUrl: './page-import.component.html',
  styleUrls: ['./page-import.component.scss'],
  imports: [ExaButtonComponent],
})
export class PageImportComponent {
  private readonly store = inject(Store);
  private readonly router = inject(Router);
  private readonly pages = this.store.selectSignal(ImportsState.pages);
  private readonly summary = this.store.selectSignal(DeploymentsState.selectedSummary);
  private readonly operations = this.store.selectSignal(OperationsState.operations);

  readonly name = input.required<string>();
  readonly page = computed(() => this.pages()[this.name()] ?? emptyImportPage());
  readonly ready = computed(() => this.summary()?.status === DEPLOYMENT_READY_STATUS);
  readonly stopped = computed(() => !!this.summary() && !this.ready());
  readonly operation = computed(() => {
    const latest = this.operations().find(operation => operation.deployment === this.name());
    return latest?.status === OperationStatus.RUNNING ? latest : undefined;
  });
  readonly canStart = computed(() => !this.operation() && this.summary()?.status === 'stopped');
  readonly canChoose = computed(() => this.ready() && this.page().phase !== 'uploading');
  readonly dragging = signal(false);

  readonly operationVerb = RUNNING_VERB;
  readonly status = ImportStatus;
  readonly count = formatCount;
  readonly seconds = formatSeconds;
  readonly percent = (fraction: number) => Math.round(fraction * PERCENT);

  constructor() {
    effect(() => {
      const name = this.name();
      untracked(() => {
        this.store.dispatch(new SelectDeploymentAction(name));
        this.store.dispatch(new LoadImportsAction(name));
      });
    });
    this.refreshWhenOperationEnds();
  }

  start(): void {
    this.store.dispatch(new RunOperationAction(this.name(), OperationKind.START));
  }

  dragOver(event: DragEvent): void {
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = this.canChoose() ? 'copy' : 'none';
    this.dragging.set(this.canChoose());
  }

  dropped(event: DragEvent): void {
    event.preventDefault();
    this.dragging.set(false);
    this.upload(Array.from(event.dataTransfer?.files ?? []));
  }

  chosen(input: HTMLInputElement): void {
    this.upload(Array.from(input.files ?? []));
    input.value = '';
  }

  /** Opens the SQL editor with a query on the imported table below its text. */
  query(entry: DataImport): void {
    appendText(this.name(), queryStatement(entry.schema, entry.table));
    void this.router.navigate(deploymentPath(this.name(), DeploymentPage.SQL));
  }

  private upload(files: File[]): void {
    if (files.length > 0 && this.canChoose()) this.store.dispatch(new UploadFilesAction(this.name(), files));
  }

  private refreshWhenOperationEnds(): void {
    let previous: Operation | undefined;
    effect(() => {
      const operation = this.operation();
      if (previous && !operation) untracked(() => this.store.dispatch(new LoadDeploymentsAction(true)));
      previous = operation;
    });
  }
}
