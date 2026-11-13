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
  DEPLOY: 'deploy',
  DESTROY: 'destroy',
  DESTROY_AND_REMOVE: 'destroy_and_remove',
  REMOVE: 'remove',
  ENABLE_SIDECAR: 'enable_sidecar',
  DISABLE_SIDECAR: 'disable_sidecar',
  REMOVE_SIDECAR: 'remove_sidecar',
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
  /** The sidecar that sidecar operations act on. */
  sidecar?: string;
  status: OperationStatus;
  started_at: string;
  finished_at?: string;
  result?: { deployment_state: string; database_ready: boolean };
  error?: string;
  lines?: string[];
}

export type OptionValue = string | number | boolean;

export interface ConfigurationOption {
  name: string;
  /** Absent for secret options, whose values never reach the browser. */
  value?: OptionValue;
  secret: boolean;
}

export interface ConfigurationPreset {
  name: string;
  display_name: string;
  description: string;
  options: ConfigurationOption[];
}

export const PresetKind = {
  INFRASTRUCTURE: 'infrastructure',
  INSTALLATION: 'installation',
} as const;
export type PresetKind = (typeof PresetKind)[keyof typeof PresetKind];

export interface Configuration {
  infrastructure: ConfigurationPreset;
  installation: ConfigurationPreset;
  editable: boolean;
  apply_with?: 'start' | 'deploy';
}

export const SidecarHealth = {
  SERVING: 'serving',
  STARTING: 'starting',
  FAILING: 'failing',
} as const;
export type SidecarHealth = (typeof SidecarHealth)[keyof typeof SidecarHealth] | '';

export interface SidecarEndpoint {
  ip: string;
  port: number;
  url?: string;
}

export interface Sidecar {
  name: string;
  description?: string;
  catalog: boolean;
  enabled: boolean;
  running: boolean;
  state: string;
  health: SidecarHealth;
  health_error?: string;
  error?: string;
  last_operation_error?: string;
  restart_required: boolean;
  endpoints: SidecarEndpoint[];
}
