import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { Store } from '@ngxs/store';
import { filter } from 'rxjs';
import { ClipboardCopyButtonComponent, ExaButtonComponent } from '@exasol/ui-common';
import { OperationProgressComponent } from '@exa/components/operation-progress/operation-progress.component';
import { SidecarDisableDialogComponent } from '@exa/components/sidecar-disable-dialog/sidecar-disable-dialog.component';
import { MCP_SIDECAR, OperationKind, SidecarHealth } from '@exa/services/local-api/local-api.models';
import { runningLabel } from '@exa/store/operations/operation-labels';
import { RunOperationAction } from '@exa/store/operations/operations.actions';
import { followSidecars } from '@exa/store/sidecars/sidecar-page';
import { LoadMcpClientsAction, LoadMcpInstructionsAction } from '@exa/store/sidecars/sidecars.actions';
import { SidecarsState } from '@exa/store/sidecars/sidecars.state';

@Component({
  selector: 'exa-page-mcp',
  templateUrl: './page-mcp.component.html',
  styleUrls: ['./page-mcp.component.scss'],
  imports: [ExaButtonComponent, ClipboardCopyButtonComponent, NgTemplateOutlet, OperationProgressComponent],
})
export class PageMcpComponent {
  private readonly store = inject(Store);
  private readonly dialog = inject(MatDialog);

  readonly name = input.required<string>();

  private readonly page = followSidecars(this.name);
  readonly loadError = this.page.loadError;
  readonly running = this.page.running;
  readonly failed = this.page.failed;
  readonly deploymentStopped = this.page.deploymentStopped;
  readonly loaded = computed(() => !!this.page.sidecars());
  readonly mcp = computed(() => this.page.sidecars()?.find(sidecar => sidecar.name === MCP_SIDECAR));
  readonly enabled = computed(() => !!this.mcp()?.enabled);
  readonly endpoint = computed(() => {
    const endpoint = this.mcp()?.endpoints[0];
    return endpoint ? endpoint.url ?? `${endpoint.ip}:${endpoint.port}` : undefined;
  });

  private readonly allClients = this.store.selectSignal(SidecarsState.mcpClients);
  private readonly allInstructions = this.store.selectSignal(SidecarsState.mcpInstructions);
  private readonly mcpErrors = this.store.selectSignal(SidecarsState.mcpErrors);
  readonly clients = computed(() => this.allClients()[this.name()] ?? []);
  readonly chosenClient = signal<string | undefined>(undefined);
  readonly selectedClient = computed(() => this.chosenClient() ?? this.clients()[0]?.id);
  readonly instructions = computed(() => {
    const instructions = this.allInstructions()[this.name()];
    return instructions?.id === this.selectedClient() ? instructions : undefined;
  });
  readonly instructionsError = computed(() => this.mcpErrors()[this.name()]);

  readonly health = SidecarHealth;
  readonly runningLabel = runningLabel;

  constructor() {
    effect(() => {
      this.name();
      untracked(() => this.chosenClient.set(undefined));
    });
    effect(() => {
      const name = this.name();
      if (this.enabled()) {
        untracked(() => this.store.dispatch(new LoadMcpClientsAction(name)));
      }
    });
    effect(() => {
      const client = this.selectedClient();
      if (this.enabled() && client) {
        const name = untracked(this.name);
        untracked(() => this.store.dispatch(new LoadMcpInstructionsAction(name, client)));
      }
    });
  }

  choose(client: string): void {
    this.chosenClient.set(client);
  }

  enable(): void {
    this.store.dispatch(new RunOperationAction(this.name(), OperationKind.ENABLE_SIDECAR, { sidecar: MCP_SIDECAR }));
  }

  disable(): void {
    const deployment = this.name();
    SidecarDisableDialogComponent.open(this.dialog, { deployment, sidecar: MCP_SIDECAR })
      .pipe(filter(kind => !!kind))
      .subscribe(kind => this.store.dispatch(new RunOperationAction(deployment, kind, { sidecar: MCP_SIDECAR })));
  }
}
