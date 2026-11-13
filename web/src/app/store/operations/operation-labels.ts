import { Operation, OperationKind, OperationStatus } from '@exa/services/local-api/local-api.models';

const RUNNING_VERB: Record<OperationKind, string> = { start: 'Starting', stop: 'Stopping' };
const DONE_VERB: Record<OperationKind, string> = { start: 'Started', stop: 'Stopped' };
const NOUN: Record<OperationKind, string> = { start: 'Start', stop: 'Stop' };

/** Names an operation as the activity indicator and panel show it. */
export const operationLabel = (operation: Operation): string => {
  switch (operation.status) {
    case OperationStatus.RUNNING:
      return `${RUNNING_VERB[operation.kind]} ${operation.deployment}`;
    case OperationStatus.SUCCEEDED:
      return `${DONE_VERB[operation.kind]} ${operation.deployment}`;
    default:
      return `${NOUN[operation.kind]} of ${operation.deployment} failed`;
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
