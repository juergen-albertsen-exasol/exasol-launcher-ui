import { HttpClient, HttpErrorResponse, HttpParams, HttpResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import {
  Configuration,
  DeploymentDetails,
  DeploymentSummary,
  DiagnosticReport,
  LocalRuntime,
  McpClient,
  McpInstructions,
  Operation,
  OperationKind,
  OperationParameters,
  OptionValue,
  PingResult,
  Problem,
  SchemaSummary,
  ScriptLanguages,
  Sidecar,
  SqlSession,
  StatementOutcome,
} from './local-api.models';

export const API_BASE_PATH = '/api/v1';

@Injectable({ providedIn: 'root' })
export class LocalApiService {
  private readonly http = inject(HttpClient);

  listDeployments(refresh = false): Observable<DeploymentSummary[]> {
    const params = refresh ? new HttpParams().set('refresh', 'true') : undefined;
    return this.http.get<DeploymentSummary[]>(`${API_BASE_PATH}/deployments`, { params });
  }

  getDeployment(name: string): Observable<DeploymentDetails> {
    return this.http.get<DeploymentDetails>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}`);
  }

  openFolder(name: string): Observable<void> {
    return this.http.post<void>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/open-folder`, null);
  }

  runOperation(name: string, kind: OperationKind, params: OperationParameters = {}): Observable<Operation> {
    const deployment = `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}`;
    const sidecarPath = `${deployment}/sidecars/${encodeURIComponent(params.sidecar ?? '')}`;
    const scriptLanguages = `${deployment}/script-languages`;
    const scriptLanguagePath = `${scriptLanguages}/${encodeURIComponent(params.script_language ?? '')}`;
    const restart = { restart: params.restart ?? true };
    const custom = { alias: params.script_language, language: params.language, source: params.source, ...restart };
    switch (kind) {
      case 'destroy_and_remove':
        return this.http.post<Operation>(`${deployment}/destroy`, { remove_folder: true });
      case 'enable_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/enable`, null);
      case 'disable_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/disable`, null);
      case 'remove_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/disable`, { remove_definition: true });
      case 'install_script_language':
        return this.http.post<Operation>(`${scriptLanguagePath}/install`, restart);
      case 'update_script_language':
        return this.http.post<Operation>(`${scriptLanguagePath}/update`, restart);
      case 'remove_script_language':
        return this.http.post<Operation>(`${scriptLanguagePath}/remove`, restart);
      case 'install_custom_script_language':
        return this.http.post<Operation>(`${scriptLanguages}/custom/install`, custom);
      case 'update_custom_script_language':
        return this.http.post<Operation>(`${scriptLanguages}/custom/update`, custom);
      default:
        return this.http.post<Operation>(`${deployment}/${kind}`, null);
    }
  }

  getConfiguration(name: string): Observable<Configuration> {
    return this.http.get<Configuration>(this.configurationPath(name));
  }

  setConfiguration(name: string, options: { [option: string]: OptionValue }): Observable<Configuration> {
    return this.http.patch<Configuration>(this.configurationPath(name), { options });
  }

  resetConfiguration(name: string, options: string[]): Observable<Configuration> {
    return this.http.post<Configuration>(`${this.configurationPath(name)}/reset`, { options });
  }

  private configurationPath(name: string): string {
    return `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/configuration`;
  }

  listSidecars(name: string): Observable<Sidecar[]> {
    return this.http.get<Sidecar[]>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/sidecars`);
  }

  listScriptLanguages(name: string): Observable<ScriptLanguages> {
    return this.http.get<ScriptLanguages>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/script-languages`);
  }

  listMcpClients(name: string): Observable<McpClient[]> {
    return this.http.get<McpClient[]>(this.mcpClientsPath(name));
  }

  getMcpInstructions(name: string, client: string): Observable<McpInstructions> {
    return this.http.get<McpInstructions>(`${this.mcpClientsPath(name)}/${encodeURIComponent(client)}`);
  }

  private mcpClientsPath(name: string): string {
    return `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/mcp/clients`;
  }

  getLocalRuntime(name: string): Observable<LocalRuntime> {
    return this.http.get<LocalRuntime>(`${this.diagnosticsPath(name)}/local`);
  }

  getDiagnosticReport(name: string): Observable<DiagnosticReport> {
    return this.http.get<DiagnosticReport>(`${this.diagnosticsPath(name)}/info`);
  }

  unlock(name: string): Observable<void> {
    return this.http.post<void>(`${this.diagnosticsPath(name)}/unlock`, null);
  }

  private diagnosticsPath(name: string): string {
    return `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/diagnostics`;
  }

  listOperations(): Observable<Operation[]> {
    return this.http.get<Operation[]>(`${API_BASE_PATH}/operations`);
  }

  getOperation(id: string): Observable<Operation> {
    return this.http.get<Operation>(`${API_BASE_PATH}/operations/${encodeURIComponent(id)}`);
  }

  openSqlSession(name: string): Observable<SqlSession> {
    return this.http.post<SqlSession>(this.sqlSessionsPath(name), null);
  }

  /** Closes a session with a request that outlives the page, so it also works while the page unloads. */
  closeSqlSession(name: string, id: string): void {
    fetch(this.sqlSessionPath(name, id), { method: 'DELETE', keepalive: true, credentials: 'same-origin' }).catch(
      () => undefined
    );
  }

  runStatement(name: string, id: string, sql: string): Observable<StatementOutcome> {
    return this.http.post<StatementOutcome>(`${this.sqlSessionPath(name, id)}/statements`, { sql });
  }

  cancelStatement(name: string, id: string): Observable<void> {
    return this.http.post<void>(`${this.sqlSessionPath(name, id)}/cancel`, null);
  }

  /** Returns the URL that downloads the statement's full result as CSV once. */
  createExport(name: string, id: string, sql: string): Observable<string> {
    return this.http
      .post(`${this.sqlSessionPath(name, id)}/exports`, { sql }, { observe: 'response' })
      .pipe(map((response: HttpResponse<unknown>) => response.headers.get('Location') ?? ''));
  }

  private sqlSessionsPath(name: string): string {
    return `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/sql/sessions`;
  }

  private sqlSessionPath(name: string, id: string): string {
    return `${this.sqlSessionsPath(name)}/${encodeURIComponent(id)}`;
  }

  listSchemas(name: string): Observable<SchemaSummary[]> {
    return this.http.get<SchemaSummary[]>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/schemas`);
  }

  getSchema(name: string, schema: string): Observable<SchemaSummary> {
    return this.http.get<SchemaSummary>(
      `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/schemas/${encodeURIComponent(schema)}`
    );
  }

  pingDeployment(name: string): Observable<PingResult> {
    return this.http.get<PingResult>(`${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/health/ping`);
  }
}

export const problemDetail = (error: unknown): string => {
  if (error instanceof HttpErrorResponse) {
    const problem = error.error as Partial<Problem> | null;
    return problem?.detail ?? problem?.title ?? error.message;
  }
  return error instanceof Error ? error.message : String(error);
};
