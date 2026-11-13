export const DEPLOYMENT_READY_STATUS = 'database_ready';

export interface DeploymentSummary {
  name: string;
  path: string;
  status: string;
  infrastructure: string;
  installation: string;
}

export interface ConnectionDetails {
  host: string;
  port: number;
  username: string;
  insecure_skip_cert_validation: boolean;
}

export interface DeploymentDetails {
  name: string;
  state: string;
  connection?: ConnectionDetails;
}

export const PingStatus = {
  REACHABLE: 'reachable',
  UNREACHABLE: 'unreachable',
} as const;
export type PingStatus = (typeof PingStatus)[keyof typeof PingStatus];

export interface PingResult {
  status: PingStatus;
  round_trip_ms?: number;
  error?: string;
}

export interface Problem {
  type: string;
  title: string;
  detail?: string;
}

export const OperationKind = {
  START: 'start',
  STOP: 'stop',
} as const;
export type OperationKind = (typeof OperationKind)[keyof typeof OperationKind];

export const OperationStatus = {
  RUNNING: 'running',
  SUCCEEDED: 'succeeded',
  FAILED: 'failed',
} as const;
export type OperationStatus = (typeof OperationStatus)[keyof typeof OperationStatus];

export interface Operation {
  id: string;
  deployment: string;
  kind: OperationKind;
  status: OperationStatus;
  started_at: string;
  finished_at?: string;
  result?: { deployment_state: string; database_ready: boolean };
  error?: string;
  lines?: string[];
}
