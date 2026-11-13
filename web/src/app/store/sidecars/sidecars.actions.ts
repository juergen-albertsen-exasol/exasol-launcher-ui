export class LoadSidecarsAction {
  static readonly type = '[Sidecars] Load';
  constructor(public name: string) {}
}

export class LoadMcpClientsAction {
  static readonly type = '[Sidecars] Load MCP clients';
  constructor(public name: string) {}
}

export class LoadMcpInstructionsAction {
  static readonly type = '[Sidecars] Load MCP instructions';
  constructor(
    public name: string,
    public client: string
  ) {}
}
