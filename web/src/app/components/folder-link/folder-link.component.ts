import { Component, computed, inject, input } from '@angular/core';
import { Store } from '@ngxs/store';
import { ErrorNotificationsAction } from '@exasol/ui-common';
import { LocalApiService, problemDetail } from '@exa/services/local-api/local-api.service';

/** Converts a local path, POSIX or Windows, into its file:// URL. */
export const fileUrl = (path: string): string => {
  const slashed = path.replace(/\\/g, '/');
  const absolute = /^[A-Za-z]:\//.test(slashed) ? `/${slashed}` : slashed;
  return `file://${encodeURI(absolute)}`;
};

@Component({
  selector: 'exa-folder-link',
  templateUrl: './folder-link.component.html',
  styleUrls: ['./folder-link.component.scss'],
})
export class FolderLinkComponent {
  private readonly api = inject(LocalApiService);
  private readonly store = inject(Store);

  readonly deployment = input.required<string>();
  readonly path = input.required<string>();
  readonly href = computed(() => fileUrl(this.path()));

  // Browsers block file:// navigation from an http:// page, so the server
  // opens the folder; the href stays for copying the address.
  open(event: MouseEvent): void {
    event.preventDefault();
    this.api.openFolder(this.deployment()).subscribe({
      error: (error: unknown) =>
        this.store.dispatch(new ErrorNotificationsAction(`Cannot open the folder: ${problemDetail(error)}`)),
    });
  }
}
