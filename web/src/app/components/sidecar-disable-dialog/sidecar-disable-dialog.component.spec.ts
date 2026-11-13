import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { By } from '@angular/platform-browser';
import { SidecarDisableDialogComponent } from './sidecar-disable-dialog.component';

describe('SidecarDisableDialogComponent', () => {
  let fixture: ComponentFixture<SidecarDisableDialogComponent>;
  let close: jest.Mock;

  beforeEach(async () => {
    close = jest.fn();
    await TestBed.configureTestingModule({
      imports: [SidecarDisableDialogComponent],
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: { deployment: 'default', sidecar: 'kafka' } },
        { provide: MatDialogRef, useValue: { close } },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(SidecarDisableDialogComponent);
    fixture.detectChanges();
  });

  const element = <T extends HTMLElement>(selector: string) =>
    fixture.debugElement.query(By.css(selector)).nativeElement as T;

  it('disables the sidecar and keeps its definition by default', () => {
    expect(element('.sidecar-disable-dialog__title').textContent?.trim()).toBe('Disable kafka on default?');

    element<HTMLButtonElement>('.sidecar-disable-dialog__confirm').click();

    expect(close).toHaveBeenCalledWith('disable_sidecar');
  });

  it('deletes the saved definition when the checkbox is checked', () => {
    element<HTMLInputElement>('.sidecar-disable-dialog__remove').click();
    element<HTMLButtonElement>('.sidecar-disable-dialog__confirm').click();

    expect(close).toHaveBeenCalledWith('remove_sidecar');
  });

  it('closes without a result on cancel', () => {
    element<HTMLButtonElement>('.sidecar-disable-dialog__cancel').click();

    expect(close).toHaveBeenCalledWith();
  });
});
