import type { ImportColumn } from '@exa/services/local-api/local-api.models';

export class LoadImportsAction {
  static readonly type = '[Imports] Load';
  constructor(public name: string) {}
}

export class UploadFilesAction {
  static readonly type = '[Imports] Upload files';
  constructor(
    public name: string,
    public files: File[]
  ) {}
}

export class EditImportTableAction {
  static readonly type = '[Imports] Edit table';
  constructor(
    public name: string,
    public table: { schema?: string; table?: string }
  ) {}
}

export class EditImportColumnAction {
  static readonly type = '[Imports] Edit column';
  constructor(
    public name: string,
    public index: number,
    public column: Partial<ImportColumn>
  ) {}
}

export class ResetImportAction {
  static readonly type = '[Imports] Reset to the suggestion';
  constructor(public name: string) {}
}

export class StartImportAction {
  static readonly type = '[Imports] Start';
  constructor(public name: string) {}
}

export class RefreshImportAction {
  static readonly type = '[Imports] Refresh';
  constructor(public name: string) {}
}

export class CancelImportAction {
  static readonly type = '[Imports] Cancel';
  constructor(public name: string) {}
}

export class ChooseFilesAgainAction {
  static readonly type = '[Imports] Choose files again';
  constructor(public name: string) {}
}
