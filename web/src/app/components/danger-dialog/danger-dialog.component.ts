import { Component, computed, inject, signal } from '@angular/core';
import { MAT_DIALOG_DATA, MatDialog, MatDialogRef } from '@angular/material/dialog';
import { Observable } from 'rxjs';
import { OperationKind } from '@exa/services/local-api/local-api.models';

export interface DangerDialogData {
  deployment: string;
  path: string;
  /** Destroy offers to remove the folder too; remove only removes the folder. */
  action: 'destroy' | 'remove';
  removeFolder?: boolean;
}

type DangerKind = typeof OperationKind.DESTROY | typeof OperationKind.DESTROY_AND_REMOVE | typeof OperationKind.REMOVE;

/** Confirms a destructive operation once the user types the deployment name. */
@Component({
  selector: 'exa-danger-dialog',
  templateUrl: './danger-dialog.component.html',
  styleUrls: ['./danger-dialog.component.scss'],
})
export class DangerDialogComponent {
  private readonly dialogRef = inject<MatDialogRef<DangerDialogComponent, DangerKind>>(MatDialogRef);
  readonly data = inject<DangerDialogData>(MAT_DIALOG_DATA);

  readonly removeFolder = signal(this.data.action === 'remove' || !!this.data.removeFolder);
  readonly typed = signal('');
  readonly confirmed = computed(() => this.typed() === this.data.deployment);

  static open(dialog: MatDialog, data: DangerDialogData): Observable<DangerKind | undefined> {
    return dialog
      .open<DangerDialogComponent, DangerDialogData, DangerKind>(DangerDialogComponent, {
        data,
        width: '520px',
        maxWidth: 'calc(100vw - 32px)',
        panelClass: 'exa-danger-dialog',
        autoFocus: 'input',
      })
      .afterClosed();
  }

  confirm(): void {
    if (!this.confirmed()) {
      return;
    }
    if (this.data.action === 'remove') {
      this.dialogRef.close(OperationKind.REMOVE);
    } else {
      this.dialogRef.close(this.removeFolder() ? OperationKind.DESTROY_AND_REMOVE : OperationKind.DESTROY);
    }
  }

  cancel(): void {
    this.dialogRef.close();
  }
}
