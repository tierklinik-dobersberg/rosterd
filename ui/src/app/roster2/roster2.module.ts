import { DragDropModule } from "@angular/cdk/drag-drop";
import { OverlayModule } from "@angular/cdk/overlay";
import { CommonModule } from "@angular/common";
import { NgModule } from "@angular/core";
import { FormsModule, ReactiveFormsModule } from "@angular/forms";
import { RouterModule } from "@angular/router";
import { NgIconsModule } from "@ng-icons/core";
import { lucideListPlus } from '@ng-icons/lucide';
import { BrnAlertDialogModule } from '@spartan-ng/ui-alertdialog-brain';
import { BrnDialogModule } from '@spartan-ng/ui-dialog-brain';
import { BrnMenuTriggerDirective } from '@spartan-ng/ui-menu-brain';
import { BrnSelectModule } from '@spartan-ng/ui-select-brain';
import { BrnTableModule } from '@spartan-ng/ui-table-brain';
import { HlmAlertDialogModule } from '@tierklinik-dobersberg/angular/alertdialog';
import { HlmButtonModule } from '@tierklinik-dobersberg/angular/button';
import { HlmDialogModule } from '@tierklinik-dobersberg/angular/dialog';
import { HlmIconModule, provideIcons } from '@tierklinik-dobersberg/angular/icon';
import { HlmInputModule } from '@tierklinik-dobersberg/angular/input';
import { HlmLabelModule } from '@tierklinik-dobersberg/angular/label';
import { HlmMenuModule } from '@tierklinik-dobersberg/angular/menu';
import { HlmAvatarModule } from '@tierklinik-dobersberg/angular/avatar';
import { DisplayNamePipe, DurationPipe, TkdInListPipe, ToUserPipe, UserColorPipe, UserContrastColorPipe } from "@tierklinik-dobersberg/angular/pipes";
import { HlmSelectModule } from '@tierklinik-dobersberg/angular/select';
import { HlmH1Directive, HlmH2Directive, HlmH3Directive } from '@tierklinik-dobersberg/angular/typography';
import { HlmTableModule } from '@tierklinik-dobersberg/angular/table';
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzBadgeModule } from 'ng-zorro-antd/badge';
import { NzCalendarModule } from 'ng-zorro-antd/calendar';
import { NzDatePickerModule } from "ng-zorro-antd/date-picker";
import { NzDrawerModule } from "ng-zorro-antd/drawer";
import { NzDropDownModule } from "ng-zorro-antd/dropdown";
import { NzEmptyModule } from "ng-zorro-antd/empty";
import { NZ_DATE_CONFIG } from "ng-zorro-antd/i18n";
import { NzMessageModule } from "ng-zorro-antd/message";
import { NzModalModule } from "ng-zorro-antd/modal";
import { NzSelectModule } from "ng-zorro-antd/select";
import { NzToolTipModule } from 'ng-zorro-antd/tooltip';
import { ApprovalComponent } from './approval/approval.component';
import { TkdConstraintViolationPipe } from "./constraint-violation-text.pipe";
import { TkdDebounceEventDirective } from "./debounce-event.directive";
import { TkdRosterOverviewComponent } from "./overview/overview.component";
import { TkdRosterPlannerComponent, TkdRosterPlannerDayComponent } from './planner';
import { TkdRoster2Routing } from './roster2-routing.module';
import { UserLetterPipe } from 'src/app/common/pipes';
import { TkdTableSortColumnComponent } from "../common/table-sort";

@NgModule({
  imports: [
    TkdRoster2Routing,
    NzCalendarModule,
    NzAvatarModule,
    NzBadgeModule,
    DragDropModule,
    OverlayModule,
    NzToolTipModule,
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    NzMessageModule,
    NzModalModule,
    NzEmptyModule,
    RouterModule,
    NzDropDownModule,
    NzSelectModule,
    NzDatePickerModule,
    DurationPipe,
    ToUserPipe,
    DisplayNamePipe,
    UserColorPipe,
    UserContrastColorPipe,
    TkdInListPipe,
    NzDrawerModule,
    NgIconsModule,
    UserLetterPipe,

    HlmButtonModule,
    HlmIconModule,
    HlmMenuModule,
    HlmAlertDialogModule,
    HlmDialogModule,
    HlmInputModule,
    HlmLabelModule,
    HlmSelectModule,
    HlmTableModule,
    HlmAvatarModule,
    HlmH1Directive,
    HlmH2Directive,
    HlmH3Directive,

    BrnMenuTriggerDirective,
    BrnAlertDialogModule,
    BrnDialogModule,
    BrnSelectModule,
    BrnTableModule,
    TkdTableSortColumnComponent,
  ],
  declarations: [
    TkdRosterPlannerComponent,
    TkdRosterPlannerDayComponent,
    TkdConstraintViolationPipe,
    TkdDebounceEventDirective,
    TkdRosterOverviewComponent,
    ApprovalComponent
  ],
  exports: [
    ToUserPipe,
    DisplayNamePipe,
    DurationPipe,
  ],
  providers: [
    { provide: NZ_DATE_CONFIG, useValue: { firstDayOfWeek: 1 } },
    ...provideIcons({lucideListPlus}),
  ]
})
export class TkdRoster2Module {}
