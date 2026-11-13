import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { Store } from '@ngxs/store';
import { ClipboardCopyButtonComponent, ConfirmationModalComponent } from '@exasol/ui-common';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import {
  DiagnosticReport,
  LocalRuntime,
  Operation,
  OperationStatus,
  PortReachability,
} from '@exa/services/local-api/local-api.models';
import { LoadDeploymentsAction, SelectDeploymentAction } from '@exa/store/deployments/deployments.actions';
import { DeploymentsState } from '@exa/store/deployments/deployments.state';
import {
  LoadDiagnosticReportAction,
  LoadLocalRuntimeAction,
  UnlockDeploymentAction,
} from '@exa/store/diagnostics/diagnostics.actions';
import { DiagnosticsState } from '@exa/store/diagnostics/diagnostics.state';
import { runningLabel } from '@exa/store/operations/operation-labels';
import { OperationsState } from '@exa/store/operations/operations.state';

const LOCAL_INFRASTRUCTURE = 'local';

/** Quotes a deployment name for a POSIX shell when it holds more than safe characters. */
export const shellArgument = (value: string): string =>
  /^[A-Za-z0-9._@%+=:,/-]+$/.test(value) ? value : `'${value.replace(/'/g, `'\\''`)}'`;

export interface ShellCommand {
  lead: string;
  command: string;
}

/** Saves text as a file through a temporary download link. */
const saveFile = (fileName: string, text: string): void => {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = fileName;
  link.click();
  URL.revokeObjectURL(url);
};

@Component({
  selector: 'exa-page-diagnostics',
  templateUrl: './page-diagnostics.component.html',
  styleUrls: ['./page-diagnostics.component.scss'],
  imports: [ClipboardCopyButtonComponent, NgTemplateOutlet, OperationProgressComponent],
})
export class PageDiagnosticsComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);

  readonly name = input.required<string>();

  private readonly deployments = this.store.selectSignal(DeploymentsState.deployments);
  private readonly localRuntimes = this.store.selectSignal(DiagnosticsState.localRuntimes);
  private readonly localRuntimeErrors = this.store.selectSignal(DiagnosticsState.localRuntimeErrors);
  private readonly allChecking = this.store.selectSignal(DiagnosticsState.checking);
  private readonly reports = this.store.selectSignal(DiagnosticsState.reports);
  private readonly reportErrors = this.store.selectSignal(DiagnosticsState.reportErrors);
  private readonly operations = this.store.selectSignal(OperationsState.operations);

  readonly local = computed(
    () =>
      this.deployments()?.find(deployment => deployment.name === this.name())?.infrastructure === LOCAL_INFRASTRUCTURE
  );
  readonly localRuntime = computed<LocalRuntime | undefined>(() => this.localRuntimes()[this.name()]);
  readonly localRuntimeError = computed(() => this.localRuntimeErrors()[this.name()]);
  readonly checking = computed(() => !!this.allChecking()[this.name()]);

  readonly reportShown = signal(false);
  private readonly report = computed<DiagnosticReport | undefined>(() => this.reports()[this.name()]);
  readonly reportText = computed(() => {
    const report = this.report();
    return report ? JSON.stringify(report, null, 2) : undefined;
  });
  readonly reportError = computed(() => this.reportErrors()[this.name()]);

  private readonly latest = computed(() => this.operations().find(operation => operation.deployment === this.name()));
  readonly running = computed(() => {
    const operation = this.latest();
    return operation?.status === OperationStatus.RUNNING ? operation : undefined;
  });
  readonly failed = computed(() => {
    const operation = this.latest();
    return operation?.status === OperationStatus.FAILED ? operation : undefined;
  });

  readonly shellCommands = computed<ShellCommand[]>(() => {
    const deployment = shellArgument(this.name());
    return [
      { lead: 'Open a shell on the host node:', command: `exasol shell host -d ${deployment}` },
      { lead: 'Run one command on the host node:', command: `exasol shell host -d ${deployment} -- [command]` },
      { lead: 'Open a shell in the database container:', command: `exasol shell container -d ${deployment}` },
    ];
  });

  readonly reachable = PortReachability.REACHABLE;
  readonly runningLabel = runningLabel;

  constructor() {
    effect(() => {
      const deployment = this.name();
      untracked(() => {
        this.reportShown.set(false);
        this.store.dispatch(new SelectDeploymentAction(deployment));
      });
    });
    effect(() => {
      const deployment = this.name();
      if (this.local()) {
        untracked(() => this.store.dispatch(new LoadLocalRuntimeAction(deployment)));
      }
    });

    let previous: Operation | undefined;
    effect(() => {
      const operation = this.running();
      const deployment = untracked(this.name);
      if (previous?.deployment === deployment && !operation) {
        untracked(() => this.operationEnded(deployment));
      }
      previous = operation;
    });
  }

  checkAgain(): void {
    this.store.dispatch(new LoadLocalRuntimeAction(this.name()));
  }

  toggleReport(): void {
    const shown = !this.reportShown();
    this.reportShown.set(shown);
    if (shown && !this.report()) {
      this.store.dispatch(new LoadDiagnosticReportAction(this.name()));
    }
  }

  download(): void {
    const deployment = this.name();
    const save = () => {
      const text = this.reportText();
      if (text && untracked(this.name) === deployment) {
        saveFile(`${deployment}-info.json`, text);
      }
    };
    if (this.report()) {
      save();
    } else {
      this.store.dispatch(new LoadDiagnosticReportAction(deployment)).subscribe({ complete: save });
    }
  }

  unlock(): void {
    const deployment = this.name();
    ConfirmationModalComponent.open(this.dialog, {
      title: `Unlock ${deployment}?`,
      content:
        `This removes the lock of the folder of ${deployment}. Unlock it only when no launcher process works on ` +
        'this deployment; unlocking while one does can leave its files inconsistent.',
      focusOnCancel: true,
    })
      .afterClosed()
      .subscribe(confirmed => {
        if (confirmed) {
          this.store.dispatch(new UnlockDeploymentAction(deployment));
        }
      });
  }

  private operationEnded(deployment: string): void {
    this.store.dispatch(new LoadDeploymentsAction(true));
    if (this.local()) {
      this.store.dispatch(new LoadLocalRuntimeAction(deployment));
    }
  }
}
