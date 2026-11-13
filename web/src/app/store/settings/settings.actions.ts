import { OptionValue, PresetKind } from '@exa/services/local-api/local-api.models';

export class LoadConfigurationAction {
  static readonly type = '[Settings] Load configuration';
  constructor(public name: string) {}
}

export class SaveOptionsAction {
  static readonly type = '[Settings] Save options';
  constructor(
    public name: string,
    public preset: PresetKind,
    public options: { [option: string]: OptionValue }
  ) {}
}

export class ResetOptionsAction {
  static readonly type = '[Settings] Reset options';
  constructor(
    public name: string,
    public preset: PresetKind,
    public options: string[]
  ) {}
}

export class ChangesAppliedAction {
  static readonly type = '[Settings] Changes applied';
  constructor(public name: string) {}
}
