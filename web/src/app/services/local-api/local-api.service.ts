import { HttpClient, HttpErrorResponse, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { DeploymentDetails, DeploymentSummary, PingResult, Problem } from './local-api.models';

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
