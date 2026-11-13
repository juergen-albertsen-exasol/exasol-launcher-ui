import { HttpErrorResponse } from '@angular/common/http';
import { Observable, of, throwError } from 'rxjs';
import {
  Configuration,
  DataImport,
  DeploymentDetails,
  DeploymentSummary,
  DiagnosticReport,
  ImportRequest,
  ImportSource,
  ImportSuggestion,
  LocalRuntime,
  McpClient,
  McpInstructions,
  Operation,
  OperationKind,
  OperationParameters,
  PingResult,
  SchemaSummary,
  ScriptLanguages,
  Sidecar,
  SqlSession,
  StatementOutcome,
  StatementProblem,
  Upload,
} from '@exa/services/local-api/local-api.models';
import { LocalApiService } from '@exa/services/local-api/local-api.service';

export const summary = (name: string, status = 'database_ready'): DeploymentSummary => ({
  name,
  path: `/home/user/.exasol/personal/deployments/${name}`,
  status,
  infrastructure: 'local',
  installation: 'local',
});

export const runningDetails = (name: string, port = 8563): DeploymentDetails => ({
  name,
  state: 'running',
  connection: { host: '127.0.0.1', port, username: 'sys', insecure_skip_cert_validation: true },
});

export interface LocalApiFixture {
  deployments: DeploymentSummary[];
  details?: { [name: string]: DeploymentDetails };
  pings?: { [name: string]: PingResult };
  operations?: Operation[];
  configurations?: { [name: string]: Configuration };
  sidecars?: { [name: string]: Sidecar[] };
  /** Messages with which the API refuses to list a deployment's sidecars. */
  sidecarRefusals?: { [name: string]: string };
  scriptLanguages?: { [name: string]: ScriptLanguages };
  /** Messages with which the API refuses to list a deployment's script languages. */
  scriptLanguageRefusals?: { [name: string]: string };
  localRuntimes?: { [name: string]: LocalRuntime };
  /** Messages with which the API refuses to report a deployment's local runtime. */
  localRuntimeRefusals?: { [name: string]: string };
  reports?: { [name: string]: DiagnosticReport };
  /** Messages with which the API refuses to unlock a deployment's folder. */
  unlockRefusals?: { [name: string]: string };
  /** Outcomes of SQL statements by their text; other statements affect no rows. */
  sqlOutcomes?: { [sql: string]: StatementOutcome };
  /** Problems with which SQL statements fail, by their text. */
  sqlFailures?: { [sql: string]: StatementProblem };
  schemas?: { [name: string]: SchemaSummary[] };
  /** Schemas with their columns, by deployment and schema name. */
  schemaDetails?: { [name: string]: { [schema: string]: SchemaSummary } };
  /** Suggestions by upload id; others are refused as not Parquet. */
  suggestions?: { [uploadId: string]: ImportSuggestion };
  imports?: { [name: string]: DataImport[] };
}

export const resultSet = (
  columns: [string, string][],
  rows: (string | null)[][],
  overrides: Partial<StatementOutcome> = {}
): StatementOutcome =>
  ({
    type: 'result_set',
    columns: columns.map(([name, type]) => ({ name, type })),
    rows,
    total_rows: rows.length,
    truncated: false,
    duration_ms: 80,
    schema: null,
    ...overrides,
  }) as StatementOutcome;

export const statementFailure = (detail: string, code = '42000', line?: number, column?: number) =>
  ({ type: 'about:blank', title: 'Unprocessable Entity', detail, code, line, column }) as StatementProblem;

export const MCP_CLIENTS: McpClient[] = [
  { id: 'claude-code', name: 'Claude Code' },
  { id: 'claude-desktop', name: 'Claude Desktop' },
  { id: 'cursor', name: 'Cursor' },
];

export const mcpInstructions = (client: string): McpInstructions =>
  client === 'cursor'
    ? {
        id: 'cursor',
        name: 'Cursor',
        url: 'http://127.0.0.1:4896/mcp',
        file: '~/.cursor/mcp.json',
        content: '{\n  "mcpServers": {\n    "exasol-personal": { "url": "http://127.0.0.1:4896/mcp" }\n  }\n}\n',
      }
    : {
        id: client,
        name: MCP_CLIENTS.find(entry => entry.id === client)?.name ?? client,
        url: 'http://127.0.0.1:4896/mcp',
        command: 'claude mcp add --transport http --scope user exasol-personal http://127.0.0.1:4896/mcp',
      };

const conflict = (detail: string) =>
  throwError(() => new HttpErrorResponse({ status: 409, error: { title: 'Conflict', detail } }));

export const sidecar = (overrides: Partial<Sidecar> = {}): Sidecar => ({
  name: 'mcp',
  description: 'Exasol MCP server with read-oriented query tools',
  catalog: true,
  enabled: true,
  running: true,
  state: 'running',
  health: 'serving',
  restart_required: false,
  endpoints: [{ ip: '127.0.0.1', port: 4896, url: 'http://127.0.0.1:4896/mcp' }],
  ...overrides,
});

export const defaultSidecars = (): Sidecar[] => [
  sidecar(),
  sidecar({
    name: 'kafka',
    description: undefined,
    catalog: false,
    health: '',
    endpoints: [{ ip: '127.0.0.1', port: 19092 }],
  }),
  sidecar({
    name: 'dash-server',
    description: 'Dashboards built by AI agents',
    enabled: false,
    running: false,
    state: '',
    health: '',
    endpoints: [],
  }),
];

export const defaultScriptLanguages = (): ScriptLanguages => ({
  official: [
    {
      alias: 'JAVA',
      language: 'java',
      flavor: 'java-17',
      version: '11.2.0',
      aliases: ['JAVA', 'JAVA17'],
      installed: true,
      latest_release: false,
    },
    {
      alias: 'PYTHON3',
      language: 'python',
      flavor: 'python-3.12',
      version: '11.2.0',
      aliases: ['PYTHON3', 'PYTHON312'],
      installed: true,
      latest_release: false,
    },
    {
      alias: 'R',
      language: 'r',
      flavor: 'r-4.4',
      version: '11.2.0',
      aliases: ['R', 'R44'],
      installed: false,
      latest_release: false,
    },
    { alias: 'RUST', language: 'rust', flavor: '', aliases: ['RUST'], installed: false, latest_release: true },
  ],
  custom: [{ alias: 'MYPY3', language: 'python', source: '/Users/me/containers/my-python.tar.gz', available: true }],
});

export const runningLocalRuntime = (): LocalRuntime => ({
  platform: 'darwin/arm64',
  platform_supported: true,
  vm_running: true,
  database_ready: true,
  ports: [
    { name: 'db', host_port: 8563, reachability: 'reachable' },
    { name: 'sidecar mcp', host_port: 4896, reachability: 'reachable' },
  ],
});

export const stoppedLocalRuntime = (): LocalRuntime => ({
  platform: 'darwin/arm64',
  platform_supported: true,
  vm_running: false,
  message:
    'The platform is ready to run the local deployment. Run `exasol start` to start it, ' +
    'then run `exasol diag local` again for more detail.',
  ports: [],
});

export const diagnosticReport = (): DiagnosticReport => ({
  backend: 'local',
  deploymentState: 'running',
  connection: { host: '127.0.0.1', dbPort: 8563, username: 'sys' },
});

export const localConfiguration = (overrides: Partial<Configuration> = {}): Configuration => ({
  infrastructure: {
    name: 'local',
    display_name: 'Exasol Local',
    description: 'Exasol Local deployment using a macOS VM.',
    options: [
      { name: 'cpu-count', value: 2, secret: false },
      { name: 'data-size-gb', value: 100, secret: false },
      { name: 'memory-mb', value: 24576, secret: false },
      { name: 'ports', value: 'db:8563', secret: false },
    ],
  },
  installation: {
    name: 'local',
    display_name: 'Exasol Local',
    description: 'Uses the Exasol Local database container.',
    options: [],
  },
  editable: true,
  apply_with: 'start',
  ...overrides,
});

export const operation = (overrides: Partial<Operation> = {}): Operation => ({
  id: 'op-1',
  deployment: 'default',
  kind: 'start',
  status: 'running',
  started_at: new Date(Date.now() - 72_000).toISOString(),
  ...overrides,
});

export const tripsSuggestion = (overrides: Partial<ImportSuggestion> = {}): ImportSuggestion => ({
  source: { kind: 'upload', label: '2 files', files: 2, bytes: 98_400_000 },
  schema: 'DATA',
  table: 'YELLOW_TRIPDATA_2024',
  rows: 5_972_150,
  columns: [
    { name: 'VendorID', type: 'DECIMAL(10,0)', parquet_type: 'INT32', sample: '2' },
    { name: 'passenger_count', type: 'DECIMAL(20,0)', parquet_type: 'INT64', sample: '1' },
    { name: 'trip_distance', type: 'DOUBLE PRECISION', parquet_type: 'DOUBLE', sample: '1.72' },
  ],
  importable: true,
  problems: [],
  ...overrides,
});

export const dataImport = (overrides: Partial<DataImport> = {}): DataImport => ({
  id: 'import-1',
  source: { kind: 'upload', label: '2 files', files: 2, bytes: 98_400_000 },
  schema: 'DATA',
  table: 'YELLOW_TRIPDATA_2024',
  status: 'running',
  started: '2026-11-13T09:00:00Z',
  seconds: 12,
  files: [
    { name: 'yellow_tripdata_2024-01.parquet', status: 'succeeded', rows: 2_964_624 },
    { name: 'yellow_tripdata_2024-02.parquet', status: 'running' },
  ],
  sql: 'CREATE SCHEMA IF NOT EXISTS "DATA";\nCOMMIT;',
  ...overrides,
});

export const mockLocalApi = (fixture: LocalApiFixture) => ({
  listDeployments: jest.fn(() => of(fixture.deployments)),
  getDeployment: jest.fn((name: string) => of(fixture.details?.[name] ?? runningDetails(name))),
  openFolder: jest.fn(() => of(undefined)),
  listOperations: jest.fn(() => of(fixture.operations ?? [])),
  getOperation: jest.fn((id: string) => of((fixture.operations ?? []).find(op => op.id === id) ?? operation({ id }))),
  runOperation: jest.fn((name: string, kind: OperationKind, params?: OperationParameters) =>
    of(
      operation({
        id: 'new',
        deployment: name,
        kind,
        sidecar: params?.sidecar,
        script_language: params?.script_language,
      })
    )
  ),
  getConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  setConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  resetConfiguration: jest.fn((name: string) => of(fixture.configurations?.[name] ?? localConfiguration())),
  listSidecars: jest.fn((name: string) => {
    const refusal = fixture.sidecarRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.sidecars?.[name] ?? defaultSidecars());
  }),
  listScriptLanguages: jest.fn((name: string) => {
    const refusal = fixture.scriptLanguageRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.scriptLanguages?.[name] ?? defaultScriptLanguages());
  }),
  getLocalRuntime: jest.fn((name: string) => {
    const refusal = fixture.localRuntimeRefusals?.[name];
    return refusal ? conflict(refusal) : of(fixture.localRuntimes?.[name] ?? runningLocalRuntime());
  }),
  getDiagnosticReport: jest.fn((name: string) => of(fixture.reports?.[name] ?? diagnosticReport())),
  unlock: jest.fn((name: string) => {
    const refusal = fixture.unlockRefusals?.[name];
    return refusal ? conflict(refusal) : of(undefined);
  }),
  listMcpClients: jest.fn(() => of(MCP_CLIENTS)),
  getMcpInstructions: jest.fn((name: string, client: string) => of(mcpInstructions(client))),
  pingDeployment: jest.fn((name: string) => of(fixture.pings?.[name] ?? { status: 'reachable', round_trip_ms: 5 })),
  openSqlSession: jest.fn<Observable<SqlSession>, [string]>(() => of({ id: 'session-1' })),
  closeSqlSession: jest.fn<void, [string, string]>(),
  runStatement: jest.fn<Observable<StatementOutcome>, [string, string, string]>((...[, , sql]) => {
    const failure = fixture.sqlFailures?.[sql];
    if (failure) return throwError(() => new HttpErrorResponse({ status: 422, error: failure }));
    const outcome: StatementOutcome = fixture.sqlOutcomes?.[sql] ?? {
      type: 'row_count',
      row_count: 0,
      duration_ms: 10,
      schema: null,
    };
    return of(outcome);
  }),
  cancelStatement: jest.fn<Observable<void>, [string, string]>(() => of(undefined)),
  createExport: jest.fn<Observable<string>, [string, string, string]>(name =>
    of(`/api/v1/deployments/${name}/sql/exports/e1`)
  ),
  listSchemas: jest.fn((name: string) => of(fixture.schemas?.[name] ?? [])),
  createUpload: jest.fn<Observable<Upload>, [string, { name: string; size: number }[]]>((name, files) =>
    of({
      id: 'upload-1',
      files: files.map((file, index) => ({
        ...file,
        url: `/api/v1/deployments/${name}/uploads/upload-1/files/${index}`,
      })),
    })
  ),
  uploadFile: jest.fn<Observable<number>, [string, Blob]>(() => of(0.5, 1)),
  suggestImport: jest.fn<Observable<ImportSuggestion>, [string, ImportSource]>((...[, source]) => {
    const suggestion = fixture.suggestions?.[source.upload_id];
    return suggestion
      ? of(suggestion)
      : throwError(
          () =>
            new HttpErrorResponse({
              status: 422,
              error: { code: 'not_parquet', detail: 'notes.parquet is not a Parquet file, or it is damaged.' },
            })
        );
  }),
  startImport: jest.fn<Observable<DataImport>, [string, ImportRequest]>((...[, request]) =>
    of(dataImport({ id: 'import-1', schema: request.schema, table: request.table }))
  ),
  listImports: jest.fn((name: string) => of(fixture.imports?.[name] ?? [])),
  getImport: jest.fn<Observable<DataImport>, [string, string]>((name, id) =>
    of((fixture.imports?.[name] ?? []).find(found => found.id === id) ?? dataImport({ id }))
  ),
  cancelImport: jest.fn<Observable<void>, [string, string]>(() => of(undefined)),
  getSchema: jest.fn((name: string, schema: string) => {
    const details = fixture.schemaDetails?.[name]?.[schema];
    return details
      ? of(details)
      : throwError(() => new HttpErrorResponse({ status: 404, error: { detail: 'the database has no such schema' } }));
  }),
});

export const provideLocalApi = (fixture: LocalApiFixture) => ({
  provide: LocalApiService,
  useValue: mockLocalApi(fixture),
});
