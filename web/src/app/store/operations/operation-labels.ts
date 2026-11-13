import { Operation, OperationKind, OperationStatus } from '@exa/services/local-api/local-api.models';

/** What a running operation does, followed by what it acts on. */
export const RUNNING_VERB: Record<OperationKind, string> = {
  start: 'Starting',
  stop: 'Stopping',
  deploy: 'Deploying',
  destroy: 'Destroying',
  destroy_and_remove: 'Destroying and removing',
  remove: 'Removing the folder of',
  enable_sidecar: 'Enabling',
  disable_sidecar: 'Disabling',
  remove_sidecar: 'Removing',
  install_script_language: 'Installing',
  update_script_language: 'Updating',
  remove_script_language: 'Removing',
  install_custom_script_language: 'Installing',
  update_custom_script_language: 'Updating',
};
const DONE_VERB: Record<OperationKind, string> = {
  start: 'Started',
  stop: 'Stopped',
  deploy: 'Deployed',
  destroy: 'Destroyed',
  destroy_and_remove: 'Destroyed and removed',
  remove: 'Removed the folder of',
  enable_sidecar: 'Enabled',
  disable_sidecar: 'Disabled',
  remove_sidecar: 'Removed',
  install_script_language: 'Installed',
  update_script_language: 'Updated',
  remove_script_language: 'Removed',
  install_custom_script_language: 'Installed',
  update_custom_script_language: 'Updated',
};
/** What a request for an operation asks for, as in "Cannot start default". */
export const ACTION: Record<OperationKind, string> = {
  start: 'start',
  stop: 'stop',
  deploy: 'deploy',
  destroy: 'destroy',
  destroy_and_remove: 'destroy and remove',
  remove: 'remove the folder of',
  enable_sidecar: 'enable',
  disable_sidecar: 'disable',
  remove_sidecar: 'remove',
  install_script_language: 'install',
  update_script_language: 'update',
  remove_script_language: 'remove',
  install_custom_script_language: 'install',
  update_custom_script_language: 'update',
};
const NOUN: Partial<Record<OperationKind, string>> = {
  start: 'Start',
  stop: 'Stop',
  deploy: 'Deploy',
  destroy: 'Destroy',
  destroy_and_remove: 'Destroy and removal',
  remove: 'Folder removal',
};

const REMOVING: OperationKind[] = [OperationKind.REMOVE_SIDECAR, OperationKind.REMOVE_SCRIPT_LANGUAGE];

/** What an operation acts on: the deployment, or a part of it such as a sidecar. */
export const operationSubject = (deployment: string, kind: OperationKind, part?: string): string => {
  if (!part) {
    return deployment;
  }
  return `${part} ${REMOVING.includes(kind) ? 'from' : 'on'} ${deployment}`;
};

const subject = (operation: Operation): string =>
  operationSubject(operation.deployment, operation.kind, operation.sidecar ?? operation.script_language);

/** Names a running operation, as in "Starting default" or "Enabling mcp on default". */
export const runningLabel = (operation: Operation): string => `${RUNNING_VERB[operation.kind]} ${subject(operation)}`;

/** Names an operation as the activity indicator and panel show it. */
export const operationLabel = (operation: Operation): string => {
  switch (operation.status) {
    case OperationStatus.RUNNING:
      return runningLabel(operation);
    case OperationStatus.SUCCEEDED:
      return `${DONE_VERB[operation.kind]} ${subject(operation)}`;
    default: {
      const noun = NOUN[operation.kind];
      return noun ? `${noun} of ${operation.deployment} failed` : `${runningLabel(operation)} failed`;
    }
  }
};

/** Formats a duration as "45s" or "1m 12s". */
export const formatDuration = (milliseconds: number): string => {
  const seconds = Math.max(0, Math.round(milliseconds / 1000));
  const minutes = Math.floor(seconds / 60);
  return minutes ? `${minutes}m ${seconds % 60}s` : `${seconds}s`;
};

/** Formats how long ago something happened: "just now", "3 min ago", "2 h ago". */
export const formatAgo = (milliseconds: number): string => {
  const minutes = Math.floor(milliseconds / 60000);
  if (minutes < 1) {
    return 'just now';
  }
  return minutes < 60 ? `${minutes} min ago` : `${Math.floor(minutes / 60)} h ago`;
};
