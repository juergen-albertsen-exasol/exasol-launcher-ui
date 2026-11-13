import { Component, inject, signal } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialog, MatDialogRef } from '@angular/material/dialog';
import { Observable } from 'rxjs';
import { OperationKind } from '@exa/services/local-api/local-api.models';

export interface SidecarDisableDialogData {
  deployment: string;
  sidecar: string;
}

export type SidecarDisableKind = typeof OperationKind.DISABLE_SIDECAR | typeof OperationKind.REMOVE_SIDECAR;

/** Confirms disabling a sidecar and offers to delete its saved definition. */
@Component({
  selector: 'exa-sidecar-disable-dialog',
  templateUrl: './sidecar-disable-dialog.component.html',
  styleUrls: ['./sidecar-disable-dialog.component.scss'],
})
export class SidecarDisableDialogComponent {
  private readonly dialogRef = inject<MatDialogRef<SidecarDisableDialogComponent, SidecarDisableKind>>(MatDialogRef);
  readonly data = inject<SidecarDisableDialogData>(MAT_DIALOG_DATA);

  readonly removeDefinition = signal(false);

  static open(dialog: MatDialog, data: SidecarDisableDialogData): Observable<SidecarDisableKind | undefined> {
    return dialog
      .open<SidecarDisableDialogComponent, SidecarDisableDialogData, SidecarDisableKind>(
        SidecarDisableDialogComponent,
        {
          data,
          width: '480px',
          maxWidth: 'calc(100vw - 32px)',
          panelClass: 'exa-sidecar-disable-dialog',
        }
      )
      .afterClosed();
  }

  confirm(): void {
    this.dialogRef.close(this.removeDefinition() ? OperationKind.REMOVE_SIDECAR : OperationKind.DISABLE_SIDECAR);
  }

  cancel(): void {
    this.dialogRef.close();
  }
}
