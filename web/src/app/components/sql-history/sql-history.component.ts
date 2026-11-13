import { Component, ElementRef, HostListener, computed, inject, input, output, signal } from '@angular/core';
import { formatWhen, historySummary } from '@exa/sql/labels';
import { HistoryEntry, clearHistory, loadHistory } from '@exa/sql/storage';

/** The History menu of a SQL page: the statements run on the deployment in this browser. */
@Component({
  selector: 'exa-sql-history',
  templateUrl: './sql-history.component.html',
  styleUrls: ['./sql-history.component.scss'],
})
export class SqlHistoryComponent {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly deployment = input.required<string>();
  readonly pick = output<string>();

  readonly open = signal(false);
  readonly search = signal('');
  private readonly entries = signal<HistoryEntry[]>([]);
  private readonly now = signal(Date.now());

  readonly shown = computed(() => {
    const needle = this.search().trim().toLowerCase();
    return this.entries().filter(entry => entry.sql.toLowerCase().includes(needle));
  });
  readonly empty = computed(() => this.entries().length === 0);

  readonly summary = historySummary;
  when = (entry: HistoryEntry): string => formatWhen(entry.at, this.now());
  oneLine = (sql: string): string => sql.replace(/\s+/g, ' ').trim();

  toggle(): void {
    if (this.open()) {
      this.open.set(false);
      return;
    }
    this.entries.set(loadHistory(this.deployment()));
    this.now.set(Date.now());
    this.search.set('');
    this.open.set(true);
  }

  choose(entry: HistoryEntry): void {
    this.open.set(false);
    this.pick.emit(entry.sql);
  }

  clear(): void {
    clearHistory(this.deployment());
    this.entries.set([]);
  }

  searchChanged(event: Event): void {
    this.search.set((event.target as HTMLInputElement).value);
  }

  @HostListener('document:keydown.escape')
  close(): void {
    this.open.set(false);
  }

  @HostListener('document:click', ['$event'])
  closeOutside(event: MouseEvent): void {
    if (this.open() && !this.host.nativeElement.contains(event.target as Node)) this.open.set(false);
  }
}
