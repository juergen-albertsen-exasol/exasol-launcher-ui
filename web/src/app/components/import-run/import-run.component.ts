import { Component, computed, input, output } from '@angular/core';
import { ExaButtonComponent } from '@exasol/ui-common';
import { DataImport, ImportFile, ImportStatus } from '@exa/services/local-api/local-api.models';
import { formatCount, formatSeconds, queryStatement } from '@exa/store/imports/import-labels';

const PERCENT = 100;

/** A running import with its progress, or a succeeded one with what to do next. */
@Component({
  selector: 'exa-import-run',
  templateUrl: './import-run.component.html',
  styleUrls: ['./import-run.component.scss'],
  imports: [ExaButtonComponent],
})
export class ImportRunComponent {
  readonly dataImport = input.required<DataImport>();
  /** The number of columns of the new table, when known. */
  readonly columns = input<number | undefined>();
  readonly cancelImport = output<void>();
  readonly query = output<void>();
  readonly importMore = output<void>();

  readonly status = ImportStatus;
  readonly count = formatCount;
  readonly seconds = formatSeconds;

  readonly elapsed = computed(() => formatSeconds(Math.round(this.dataImport().seconds)));
  readonly tableCreated = computed(() => this.dataImport().files.some(file => file.status !== ImportStatus.WAITING));
  readonly tableLabel = computed(() => {
    const columns = this.columns();
    return columns ? `Created the table with ${columns} ${columns === 1 ? 'column' : 'columns'}` : 'Created the table';
  });
  readonly progress = computed(() => {
    const files = this.dataImport().files;
    const done = files.filter(file => file.status === ImportStatus.SUCCEEDED).length;
    return files.length === 0 ? 0 : (done / files.length) * PERCENT;
  });
  readonly queryOf = (entry: DataImport) => queryStatement(entry.schema, entry.table);

  stepIcon(status: ImportFile['status']): string {
    const icon = 'import-run__step-icon fa-solid ';
    switch (status) {
      case ImportStatus.SUCCEEDED:
        return icon + 'fa-check import-run__step-icon--done';
      case ImportStatus.RUNNING:
        return icon + 'fa-spinner fa-spin import-run__step-icon--done';
      case ImportStatus.FAILED:
      case ImportStatus.CANCELLED:
        return icon + 'fa-xmark import-run__step-icon--failed';
      default:
        return icon + 'fa-clock';
    }
  }

  fileProgress(file: ImportFile): string {
    if (file.status === ImportStatus.SUCCEEDED) return `${formatCount(file.rows ?? 0)} rows`;
    return file.status === ImportStatus.RUNNING ? 'importing' : file.status;
  }
}
