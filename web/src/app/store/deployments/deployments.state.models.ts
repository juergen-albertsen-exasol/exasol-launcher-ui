import { DeploymentDetails, DeploymentSummary, PingResult } from '@exa/services/local-api/local-api.models';

export interface DeploymentsStateModel {
  deployments?: DeploymentSummary[];
  listError?: string;
  selectedName?: string;
  details: { [name: string]: DeploymentDetails };
  detailsErrors: { [name: string]: string };
  pings: { [name: string]: PingResult };
  pingsInProgress: { [name: string]: boolean };
}
