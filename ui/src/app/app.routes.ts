import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: '/roster'},
  { path: 'admin', loadComponent: () => import("./admin/admin.component").then(m => m.AdminComponent) },
  { path: 'admin/workshifts/edit/:id', loadComponent: () => import("./admin/workshifts/workshifts.component").then(m => m.WorkshiftsComponent)},
  { path: 'admin/workshifts/new', loadComponent: () => import("./admin/workshifts/workshifts.component").then(m => m.WorkshiftsComponent), data: {isNewEntry: true}},
  { path: 'admin/constraints/edit/:id', loadComponent: () => import("./admin/constraints/constraints.component").then(m => m.ConstraintsComponent) },
  { path: 'worktimes', loadComponent: () => import("./worktimes/worktimes.component").then(m => m.WorktimesComponent) },
  { path: 'costs', loadComponent: () => import("./offtimecosts/offtimecosts.component").then(m => m.OfftimecostsComponent) },
  { path: 'costs/create', loadComponent: () => import("./offtimecosts/createcosts/createcosts.component").then(m => m.CreatecostsComponent) },
  { path: 'offtimes', loadComponent: () => import("./offtime/offtime.component").then(m => m.OfftimeComponent) },
  { path: 'roster', loadChildren: () => import("./roster2/roster2.module").then(m => m.TkdRoster2Module)},
];
