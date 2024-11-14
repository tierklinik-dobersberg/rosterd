export const environment: Config = {
  mainApplication: "https://my.dobersberg.vet",
  accountService: "https://account.dobersberg.vet",
  calendarService: "https://calendar.dobersberg.vet",
  rosterService: "https://roster.dobersberg.vet",
  commentService: "https://comments.dobersberg.vet",
  callService: "https://3cx.dobersberg.vet",
  customerService: "",
  eventService: "",
  officeHourService: "",
  orthancBridge: "",
  taskService: "",
}

import { ConnectConfig } from '@tierklinik-dobersberg/angular/connect';
interface Config  extends ConnectConfig {
  mainApplication: string
}

import 'zone.js/plugins/zone-error';
