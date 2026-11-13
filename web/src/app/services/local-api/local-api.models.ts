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
  INSTALL_SCRIPT_LANGUAGE: 'install_script_language',
  UPDATE_SCRIPT_LANGUAGE: 'update_script_language',
  REMOVE_SCRIPT_LANGUAGE: 'remove_script_language',
  INSTALL_CUSTOM_SCRIPT_LANGUAGE: 'install_custom_script_language',
  UPDATE_CUSTOM_SCRIPT_LANGUAGE: 'update_custom_script_language',
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
  /** The container alias that script language operations act on. */
  script_language?: string;
  status: OperationStatus;
  started_at: string;
  finished_at?: string;
  result?: { deployment_state: string; database_ready: boolean };
  error?: string;
  lines?: string[];
}

/** What an operation acts on besides its deployment, and how. */
export interface OperationParameters {
  sidecar?: string;
  script_language?: string;
  /** The language and source of a custom script language container. */
  language?: string;
  source?: string;
  /** Whether a script language change restarts the database now; it does unless false. */
  restart?: boolean;
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

export const MCP_SIDECAR = 'mcp';

export interface McpClient {
  id: string;
  name: string;
}

/** Registers the MCP server with a client, by running `command` or by adding `content` to `file`. */
export interface McpInstructions {
  id: string;
  name: string;
  url: string;
  command?: string;
  file?: string;
  content?: string;
  note?: string;
}

export interface OfficialScriptLanguage {
  /** The alias that identifies the container in the API. */
  alias: string;
  language: string;
  flavor: string;
  version?: string;
  aliases: string[];
  installed: boolean;
  /** Set for Rust, which installs the latest exasol-labs release instead of a catalog version. */
  latest_release: boolean;
}

export interface CustomScriptLanguage {
  alias: string;
  language: string;
  source: string;
  available: boolean;
}

export interface ScriptLanguages {
  official: OfficialScriptLanguage[];
  custom: CustomScriptLanguage[];
}

export const PortReachability = {
  REACHABLE: 'reachable',
  REFUSED: 'refused',
  BLOCKED: 'blocked',
  TIMEOUT: 'timeout',
} as const;
export type PortReachability = (typeof PortReachability)[keyof typeof PortReachability];

export interface ForwardedPort {
  /** `db`, `ui`, or a sidecar's, such as `sidecar mcp`. */
  name: string;
  host_port?: number;
  reachability: PortReachability;
}

export interface LocalRuntime {
  platform: string;
  platform_supported: boolean;
  vm_running: boolean;
  database_ready?: boolean;
  database_error?: string;
  warning?: string;
  message?: string;
  ports: ForwardedPort[];
}

/** The launcher's full report on a deployment, as `exasol diag info` prints it. */
export type DiagnosticReport = { [key: string]: unknown };

export interface SqlSession {
  id: string;
}

export interface SqlColumn {
  name: string;
  type: string;
}

export interface ResultSetOutcome {
  type: 'result_set';
  columns: SqlColumn[];
  /** Values in the database's text representation, null for NULL. */
  rows: (string | null)[][];
  total_rows: number;
  truncated: boolean;
  duration_ms: number;
  schema: string | null;
}

export interface RowCountOutcome {
  type: 'row_count';
  row_count: number;
  duration_ms: number;
  schema: string | null;
}

export type StatementOutcome = ResultSetOutcome | RowCountOutcome;

export interface StatementProblem extends Problem {
  /** The database's error code; empty when the statement failed outside the database. */
  code?: string;
  line?: number;
  column?: number;
}

export interface SchemaObject {
  name: string;
  kind: 'table' | 'view';
  columns?: SqlColumn[];
}

export interface SchemaSummary {
  name: string;
  virtual: boolean;
  objects: SchemaObject[];
}

export interface UploadFile {
  name: string;
  size: number;
  url?: string;
}

export interface Upload {
  id: string;
  files: UploadFile[];
}

export interface ImportSource {
  kind: 'upload';
  upload_id: string;
}

export interface ImportSourceSummary {
  kind: string;
  label: string;
  files: number;
  bytes: number;
}

export interface SuggestedColumn {
  name: string;
  /** The Exasol type, or null when Exasol cannot import the column. */
  type: string | null;
  parquet_type: string;
  sample: string | null;
}

export const ImportProblemCode = {
  UNSUPPORTED_COLUMNS: 'unsupported_columns',
  TABLE_TAKEN: 'table_taken',
} as const;

export interface ImportProblem {
  code: string;
  message: string;
  columns?: string[];
  alternative?: string;
}

export interface ImportSuggestion {
  source: ImportSourceSummary;
  schema: string;
  table: string;
  rows: number;
  columns: SuggestedColumn[];
  importable: boolean;
  problems: ImportProblem[];
}

export interface ImportColumn {
  name: string;
  type: string;
}

export interface ImportRequest {
  source: ImportSource;
  schema: string;
  table: string;
  columns?: ImportColumn[];
}

export const ImportStatus = {
  WAITING: 'waiting',
  RUNNING: 'running',
  SUCCEEDED: 'succeeded',
  FAILED: 'failed',
  CANCELLED: 'cancelled',
} as const;
export type ImportStatus = (typeof ImportStatus)[keyof typeof ImportStatus];

export interface ImportFile {
  name: string;
  status: ImportStatus;
  rows?: number;
}

export interface DataImport {
  id: string;
  source: ImportSourceSummary;
  schema: string;
  table: string;
  status: ImportStatus;
  started: string;
  seconds: number;
  files: ImportFile[];
  /** The total once the import succeeded. */
  rows?: number;
  /** The statements the import runs, separated by semicolons and line breaks. */
  sql: string;
  /** The database's message for a failed import. */
  message?: string;
}

export interface CodedProblem extends Problem {
  code?: string;
  alternative?: string;
  columns?: string[];
}
