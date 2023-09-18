import { NgModule } from '@angular/core';
import { Route, RouterModule } from '@angular/router';
import { ApprovalComponent } from './approval/approval.component';
import { TkdRosterOverviewComponent } from './overview/overview.component';
import { TkdRosterPlannerComponent } from './planner/roster-planner.component';


const routes: Route[] = [
  { path: '', component: TkdRosterOverviewComponent },
  { path: 'plan/:type/:from/:to', component: TkdRosterPlannerComponent },
  { path: 'view/:type/:from/:to', component: TkdRosterPlannerComponent, data: {readonly: true} },
  { path: 'plan/:id', component: TkdRosterPlannerComponent },
  { path: 'view/:id', component: TkdRosterPlannerComponent, data: {readonly: true} },
  { path: 'approve/:id', component: ApprovalComponent },
]

@NgModule({
  imports: [
    RouterModule.forChild(routes)
  ]
})
export class TkdRoster2Routing {}
