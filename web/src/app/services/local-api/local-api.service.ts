import { HttpClient, HttpErrorResponse, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import {
  Configuration,
  DeploymentDetails,
  DeploymentSummary,
  McpClient,
  McpInstructions,
  Operation,
  OperationKind,
  OptionValue,
  PingResult,
  Problem,
  Sidecar,
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

  runOperation(name: string, kind: OperationKind, sidecar = ''): Observable<Operation> {
    const deployment = `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}`;
    const sidecarPath = `${deployment}/sidecars/${encodeURIComponent(sidecar)}`;
    switch (kind) {
      case 'destroy_and_remove':
        return this.http.post<Operation>(`${deployment}/destroy`, { remove_folder: true });
      case 'enable_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/enable`, null);
      case 'disable_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/disable`, null);
      case 'remove_sidecar':
        return this.http.post<Operation>(`${sidecarPath}/disable`, { remove_definition: true });
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

  listMcpClients(name: string): Observable<McpClient[]> {
    return this.http.get<McpClient[]>(this.mcpClientsPath(name));
  }

  getMcpInstructions(name: string, client: string): Observable<McpInstructions> {
    return this.http.get<McpInstructions>(`${this.mcpClientsPath(name)}/${encodeURIComponent(client)}`);
  }

  private mcpClientsPath(name: string): string {
    return `${API_BASE_PATH}/deployments/${encodeURIComponent(name)}/mcp/clients`;
  }

  listOperations(): Observable<Operation[]> {
    return this.http.get<Operation[]>(`${API_BASE_PATH}/operations`);
  }

  getOperation(id: string): Observable<Operation> {
    return this.http.get<Operation>(`${API_BASE_PATH}/operations/${encodeURIComponent(id)}`);
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
