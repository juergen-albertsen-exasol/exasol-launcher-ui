import type { StatementToRun } from './sql.state';

export class RunStatementsAction {
  static readonly type = '[SQL] Run statements';
  constructor(
    public name: string,
    public statements: StatementToRun[]
  ) {}
}

export class CancelStatementsAction {
  static readonly type = '[SQL] Cancel statements';
  constructor(public name: string) {}
}

export class SelectResultTabAction {
  static readonly type = '[SQL] Select result tab';
  constructor(
    public name: string,
    public index: number
  ) {}
}

export class CloseSqlSessionAction {
  static readonly type = '[SQL] Close session';
  constructor(public name: string) {}
}
