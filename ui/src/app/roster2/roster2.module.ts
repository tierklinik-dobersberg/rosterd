import { DragDropModule } from "@angular/cdk/drag-drop";
import { OverlayModule } from "@angular/cdk/overlay";
import { CommonModule } from "@angular/common";
import { NgModule } from "@angular/core";
import { FormsModule } from "@angular/forms";
import { RouterModule } from "@angular/router";
import { NzAvatarModule } from 'ng-zorro-antd/avatar';
import { NzBadgeModule } from 'ng-zorro-antd/badge';
import { NzCalendarModule } from 'ng-zorro-antd/calendar';
import { NzDatePickerModule } from "ng-zorro-antd/date-picker";
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
import { DurationPipe, ToUserPipe, DisplayNamePipe, UserColorPipe, UserContrastColorPipe, TkdInListPipe } from "@tierklinik-dobersberg/angular/pipes";
import { NzDrawerModule } from "ng-zorro-antd/drawer";
import { NgIconsModule } from "@ng-icons/core";

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
    NgIconsModule
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
    { provide: NZ_DATE_CONFIG, useValue: { firstDayOfWeek: 1 } }
  ]
})
export class TkdRoster2Module {}
