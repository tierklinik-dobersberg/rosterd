import { ChangeDetectionStrategy, Component, DestroyRef, ElementRef, HostListener, Signal, ViewChild, computed, inject, input } from "@angular/core";
import { BrnSelectService } from '@spartan-ng/ui-select-brain';
import { LayoutService } from "@tierklinik-dobersberg/angular/layout";
import { HlmSheetComponent } from "@tierklinik-dobersberg/angular/sheet";
import { toDateString } from "src/utils";
import { RosterPlannerService, ShiftState } from "../planner.service";

@Component({
  // eslint-disable-next-line @angular-eslint/component-selector
  selector: 'tkd-shift-select',
  templateUrl: './shift-select.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TkdShiftSelectComponent {
  private readonly _selectService = inject(BrnSelectService, { optional: true })
  private readonly _planningService = inject(RosterPlannerService);
  private readonly _element = inject(ElementRef);
  private readonly _layout = inject(LayoutService);

  @ViewChild(HlmSheetComponent)
  protected _sheet!: HlmSheetComponent;

  @HostListener('click', ['$event'])
  protected onHostClick(event: MouseEvent) {
    const user = this._planningService.selectedUser();

    if (user) {
      event.preventDefault();
      event.stopImmediatePropagation();
      event.stopPropagation();

      if (this._selectService) {
        this._updateValueInSelect(user);
      } else {
        this._planningService.toggleUserAssignment(user, toDateString(this.shift().from), this.shift().uniqueId);
      }
    } else {
      if (!this._layout.lg() && !this._planningService.sessionState().readonly) {
        this._sheet.setSide = 'bottom'
        this._sheet.open();
      }
    }
  }

  protected onSheetUserClick(user: string) {
    this._planningService.toggleUserAssignment(user, toDateString(this.shift().from), this.shift().uniqueId);
  }

  public readonly shift = input.required<ShiftState>();

  protected readonly value: Signal<string[]>;


  protected readonly _color = computed(() => this.shift().color)
  protected readonly _displayName = computed(() => this.shift().displayName)
  protected readonly _name = computed(() => this.shift().name)
  protected readonly _staffCount = computed(() => this.shift().staffCount)
  protected readonly _profiles = computed(() => this._planningService.profiles());
  protected readonly _selectedUserConstraints = computed(() => {
    const user = this._planningService.selectedUser();
    const shift = this.shift();

    if (!user) {
      return [];
    }

    return shift.violations[user]?.violations || [];
  })

  constructor() {
    if (this._selectService) {
      const observer = new ResizeObserver(() => {
        this._selectService!.setTriggerWidth(this._element.nativeElement.offsetWidth)
      })

      observer.observe(this._element.nativeElement);

      inject(DestroyRef)
        .onDestroy(() => observer.disconnect());

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      this.value = computed(() => this._selectService!.value() as any as string[])
    } else {
      this.value = computed(() => this.shift().assignedUsers);
    }
  }

  private _updateValueInSelect(user: string) {
    if (!this._selectService) {
      return
    }

    const isAlreadyAssigned = this._selectService
      .state()
      .selectedOptions
      .find(opt => opt?.value === user);

    const opt = this._selectService.possibleOptions()
      .find(opt => opt?.value === user);

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let newOptions: any[];
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let newValue: any[];

    if (isAlreadyAssigned) {
      newOptions = this._selectService
        .state()
        .selectedOptions
        .filter(o => o !== opt)

      newValue = (this._selectService
        .state()
        .value as string[])
        .filter(o => o !== user);

      this._planningService.pushToUndoStack({
        type: 'unassign',
        dateKey: toDateString(this.shift().from),
        shiftId: this.shift().uniqueId,
        userId: user,
      })
    } else {
      newOptions = [
        ...this._selectService.state().selectedOptions,
        opt,
      ]
      newValue = [
        ...(this._selectService.state().value as string[]),
        user,
      ]

      this._planningService.pushToUndoStack({
        type: 'assign',
        dateKey: toDateString(this.shift().from),
        shiftId: this.shift().uniqueId,
        userId: user,
      })
    }

    this._selectService
      .state
      .update((state) => ({
        ...state,
        selectedOptions: newOptions,
        value: newValue
      }))
  }
}
