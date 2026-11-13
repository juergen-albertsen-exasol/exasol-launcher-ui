export class LoadLocalRuntimeAction {
  static readonly type = '[Diagnostics] Load local runtime';
  constructor(public name: string) {}
}

export class LoadDiagnosticReportAction {
  static readonly type = '[Diagnostics] Load report';
  constructor(public name: string) {}
}
