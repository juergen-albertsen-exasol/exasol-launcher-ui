import { OperationKind } from '@exa/services/local-api/local-api.models';

export class LoadOperationsAction {
  static readonly type = '[Operations] Load';
}

export class RunOperationAction {
  static readonly type = '[Operations] Run';
  constructor(
    public deployment: string,
    public kind: OperationKind
  ) {}
}
