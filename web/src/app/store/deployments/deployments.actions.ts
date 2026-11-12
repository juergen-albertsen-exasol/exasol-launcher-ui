export class LoadDeploymentsAction {
  static readonly type = '[Deployments] Load';
  constructor(public refresh = false) {}
}

export class SelectDeploymentAction {
  static readonly type = '[Deployments] Select';
  constructor(public name: string) {}
}

export class LoadDeploymentDetailsAction {
  static readonly type = '[Deployments] Load details';
  constructor(public name: string) {}
}

export class PingDeploymentAction {
  static readonly type = '[Deployments] Ping';
  constructor(public name: string) {}
}
