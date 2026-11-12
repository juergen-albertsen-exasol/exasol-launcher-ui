import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class SessionService {
  private readonly _expired = signal(false);
  readonly expired = this._expired.asReadonly();

  expire(): void {
    this._expired.set(true);
  }
}
