import { AfterViewInit, Component, ElementRef, OnDestroy, input, output, viewChild } from '@angular/core';
import {
  acceptCompletion,
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
  startCompletion,
} from '@codemirror/autocomplete';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { SQLDialect, sql } from '@codemirror/lang-sql';
import { HighlightStyle, bracketMatching, syntaxHighlighting } from '@codemirror/language';
import { highlightSelectionMatches, searchKeymap } from '@codemirror/search';
import { EditorSelection, EditorState, Prec, RangeSetBuilder, StateEffect, StateField } from '@codemirror/state';
import {
  Decoration,
  DecorationSet,
  EditorView,
  GutterMarker,
  drawSelection,
  gutterLineClass,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  placeholder,
} from '@codemirror/view';
import { tags } from '@lezer/highlight';
import { Catalog } from '@exa/sql/catalog';
import { namesCompletion } from '@exa/sql/completion';
import { FUNCTIONS, KEYWORDS, TYPES } from '@exa/sql/dialect';

export const ExasolDialect = SQLDialect.define({
  keywords: KEYWORDS.join(' '),
  types: TYPES.join(' '),
  builtin: FUNCTIONS.join(' '),
  operatorChars: '*+-%<>!=&|^/~?:',
  identifierQuotes: '"',
});

// The colors come from custom properties that the component's stylesheet
// derives from the design system, so the editor follows the theme.
const theme = EditorView.theme({
  '&': { height: '100%', backgroundColor: 'transparent', color: 'var(--text-primary)', fontSize: '13.5px' },
  '&.cm-focused': { outline: 'none' },
  '.cm-scroller': { fontFamily: 'var(--sql-font-mono)', lineHeight: '22px' },
  '.cm-content': { padding: '10px 0', caretColor: 'var(--text-primary)' },
  '.cm-gutters': { backgroundColor: 'transparent', color: 'var(--text-secondary-darken)', border: 'none' },
  '.cm-lineNumbers .cm-gutterElement': { minWidth: '34px', paddingRight: '10px' },
  '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--table-background-odd-row)' },
  '.cm-cursor': { borderLeftColor: 'var(--text-primary)', borderLeftWidth: '2px' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
    backgroundColor: 'var(--primary-accent-selected) !important',
  },
  '.cm-placeholder': { color: 'var(--text-secondary-darken)' },
  '.cm-sql-error': { textDecoration: 'underline wavy var(--sql-error)', textUnderlineOffset: '4px' },
  '.cm-sql-error-line': { color: 'var(--sql-error)', fontWeight: '600' },
  '.cm-sql-running': {
    backgroundColor: 'var(--primary-accent-lightest2)',
    boxShadow: 'inset 3px 0 0 var(--primary-accent)',
  },
  '.cm-tooltip': {
    backgroundColor: 'var(--background-secondary)',
    color: 'var(--text-primary)',
    border: '1px solid var(--frame-color)',
    borderRadius: '4px',
  },
  '.cm-tooltip-autocomplete > ul': { fontFamily: 'var(--sql-font-mono)', fontSize: '13px' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
    backgroundColor: 'var(--primary-accent-selected)',
    color: 'var(--text-primary)',
  },
  '.cm-completionDetail': { color: 'var(--text-secondary-darken)', fontStyle: 'normal', marginLeft: '1em' },
  '.cm-matchingBracket': {
    backgroundColor: 'var(--table-background-odd-row)',
    outline: '1px solid var(--frame-color)',
  },
});

const highlight = HighlightStyle.define([
  {
    tag: [tags.keyword, tags.operatorKeyword, tags.typeName, tags.standard(tags.name)],
    color: 'var(--primary-accent)',
    fontWeight: '500',
  },
  { tag: [tags.string, tags.special(tags.string)], color: 'var(--sql-string)' },
  { tag: [tags.number, tags.bool, tags.null], color: 'var(--accent-color4)' },
  {
    tag: [tags.comment, tags.lineComment, tags.blockComment],
    color: 'var(--text-secondary-darken)',
    fontStyle: 'italic',
  },
]);

interface Range {
  from: number;
  to: number;
}

const setError = StateEffect.define<Range | null>();
const setRunning = StateEffect.define<Range | null>();

/** The underlined error, which any edit clears. */
const errorField = StateField.define<Range | null>({
  create: () => null,
  update(range, transaction) {
    for (const effect of transaction.effects) if (effect.is(setError)) return effect.value;
    return transaction.docChanged ? null : range;
  },
  provide: field =>
    EditorView.decorations.from(field, range =>
      range ? Decoration.set([Decoration.mark({ class: 'cm-sql-error' }).range(range.from, range.to)]) : Decoration.none
    ),
});

const errorLineMarker = new (class extends GutterMarker {
  override elementClass = 'cm-sql-error-line';
})();

const errorGutter = gutterLineClass.compute([errorField], state => {
  const range = state.field(errorField);
  if (!range) return new RangeSetBuilder<GutterMarker>().finish();
  const line = state.doc.lineAt(range.from);
  const builder = new RangeSetBuilder<GutterMarker>();
  builder.add(line.from, line.from, errorLineMarker);
  return builder.finish();
});

/** The lines of the running statement, which follow edits. */
const runningField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(lines, transaction) {
    let next = lines.map(transaction.changes);
    for (const effect of transaction.effects) {
      if (effect.is(setRunning)) next = runningLines(transaction.state, effect.value);
    }
    return next;
  },
  provide: field => EditorView.decorations.from(field),
});

function runningLines(state: EditorState, range: Range | null): DecorationSet {
  if (!range) return Decoration.none;
  const builder = new RangeSetBuilder<Decoration>();
  const last = state.doc.lineAt(range.to).number;
  for (let number = state.doc.lineAt(range.from).number; number <= last; number++) {
    builder.add(state.doc.line(number).from, state.doc.line(number).from, Decoration.line({ class: 'cm-sql-running' }));
  }
  return builder.finish();
}

/** CodeMirror 6 with the Exasol SQL dialect, the design system's theme, and Cmd/Ctrl+Enter and Shift+Cmd/Ctrl+Enter. */
@Component({
  selector: 'exa-sql-editor',
  template: '<div #host class="sql-editor"></div>',
  styleUrls: ['./sql-editor.component.scss'],
})
export class SqlEditorComponent implements AfterViewInit, OnDestroy {
  readonly initialText = input('');
  readonly catalog = input<Catalog | undefined>(undefined);

  readonly textChange = output<string>();
  readonly run = output<void>();
  readonly runAll = output<void>();
  readonly schemasNeeded = output<string[]>();

  private readonly host = viewChild.required<ElementRef<HTMLDivElement>>('host');
  private view?: EditorView;

  ngAfterViewInit(): void {
    const language = sql({ dialect: ExasolDialect, upperCaseKeywords: true });
    this.view = new EditorView({
      parent: this.host().nativeElement,
      state: EditorState.create({
        doc: this.initialText(),
        extensions: [
          Prec.highest(
            keymap.of([
              { key: 'Mod-Enter', run: () => (this.run.emit(), true) },
              { key: 'Shift-Mod-Enter', run: () => (this.runAll.emit(), true) },
              { key: 'Ctrl-Space', run: startCompletion },
            ])
          ),
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          drawSelection(),
          bracketMatching(),
          closeBrackets(),
          highlightSelectionMatches(),
          language,
          language.language.data.of({
            autocomplete: namesCompletion(
              () => this.catalog(),
              schemas => this.schemasNeeded.emit(schemas)
            ),
          }),
          autocompletion({ activateOnTyping: true }),
          syntaxHighlighting(highlight),
          errorField,
          errorGutter,
          runningField,
          theme,
          placeholder('Write SQL here. Cmd/Ctrl+Enter runs the statement at the cursor.'),
          EditorView.contentAttributes.of({ 'aria-label': 'SQL' }),
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...searchKeymap,
            ...historyKeymap,
            ...completionKeymap,
            { key: 'Tab', run: acceptCompletion },
            indentWithTab,
          ]),
          EditorView.updateListener.of(update => {
            if (update.docChanged) this.textChange.emit(update.state.doc.toString());
          }),
        ],
      }),
    });
  }

  ngOnDestroy(): void {
    this.view?.destroy();
  }

  text(): string {
    return this.editor.state.doc.toString();
  }

  /** The selected text and its offset, or undefined when nothing is selected. */
  selection(): { text: string; from: number } | undefined {
    const range = this.editor.state.selection.main;
    return range.empty ? undefined : { text: this.editor.state.sliceDoc(range.from, range.to), from: range.from };
  }

  cursor(): number {
    return this.editor.state.selection.main.head;
  }

  /** Moves the cursor to offset, or selects from offset to `to`. */
  select(from: number, to = from): void {
    this.editor.dispatch({ selection: EditorSelection.single(from, to), scrollIntoView: true });
    this.editor.focus();
  }

  /** Replaces the selection with text, or inserts it at the cursor. */
  insert(text: string): void {
    this.editor.dispatch(this.editor.state.replaceSelection(text), { scrollIntoView: true });
    this.editor.focus();
  }

  /** Replaces the whole text, as an undoable change. */
  replaceAll(text: string): void {
    this.editor.dispatch({ changes: { from: 0, to: this.editor.state.doc.length, insert: text } });
  }

  /** Underlines the word at offset and marks its line, or clears the mark. */
  markError(offset: number | undefined): void {
    if (offset === undefined) {
      this.editor.dispatch({ effects: setError.of(null) });
      return;
    }
    const word = this.editor.state.wordAt(offset);
    const from = word?.from ?? offset;
    const to = word?.to ?? Math.min(offset + 1, this.editor.state.doc.length);
    this.editor.dispatch({ effects: setError.of(to > from ? { from, to } : null) });
  }

  /** Marks the lines of the running statement, or clears the mark. */
  markRunning(range: Range | undefined): void {
    this.editor.dispatch({ effects: setRunning.of(range ?? null) });
  }

  focus(): void {
    this.editor.focus();
  }

  private get editor(): EditorView {
    if (!this.view) throw new Error('The SQL editor is not ready.');
    return this.view;
  }
}
