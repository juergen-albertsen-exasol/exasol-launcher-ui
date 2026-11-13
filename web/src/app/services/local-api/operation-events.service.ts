import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { Operation } from './local-api.models';
import { API_BASE_PATH } from './local-api.service';

export type OperationEvent = { type: 'line'; line: string } | { type: 'status'; operation: Operation };

/** Follows an operation's progress through the server's event stream. */
@Injectable({ providedIn: 'root' })
export class OperationEventsService {
  follow(id: string): Observable<OperationEvent> {
    return new Observable<OperationEvent>(subscriber => {
      const source = new EventSource(`${API_BASE_PATH}/operations/${encodeURIComponent(id)}/events`);
      source.addEventListener('line', event =>
        subscriber.next({ type: 'line', line: JSON.parse((event as MessageEvent).data) })
      );
      source.addEventListener('status', event => {
        subscriber.next({ type: 'status', operation: JSON.parse((event as MessageEvent).data) });
        subscriber.complete();
      });
      // The stream ends after its status event; any other error ends it too.
      source.onerror = () => subscriber.complete();
      return () => source.close();
    });
  }
}
