import { operation } from '../../testing/local-api.mock';
import { formatAgo, formatDuration, operationLabel } from './operation-labels';

describe('operation labels', () => {
  it('names running, finished, and failed operations', () => {
    expect(operationLabel(operation({ kind: 'start' }))).toBe('Starting default');
    expect(operationLabel(operation({ kind: 'stop', status: 'succeeded' }))).toBe('Stopped default');
    expect(operationLabel(operation({ kind: 'start', status: 'failed' }))).toBe('Start of default failed');
    expect(operationLabel(operation({ kind: 'deploy' }))).toBe('Deploying default');
    expect(operationLabel(operation({ kind: 'destroy', status: 'succeeded' }))).toBe('Destroyed default');
    expect(operationLabel(operation({ kind: 'destroy_and_remove', status: 'failed' }))).toBe(
      'Destroy and removal of default failed'
    );
    expect(operationLabel(operation({ kind: 'remove' }))).toBe('Removing the folder of default');
  });

  it('formats durations and ages', () => {
    expect(formatDuration(45_000)).toBe('45s');
    expect(formatDuration(72_000)).toBe('1m 12s');
    expect(formatAgo(20_000)).toBe('just now');
    expect(formatAgo(180_000)).toBe('3 min ago');
    expect(formatAgo(7_200_000)).toBe('2 h ago');
  });
});
