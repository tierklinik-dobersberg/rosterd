import { signal } from "@angular/core";
import { PartialMessage } from "@bufbuild/protobuf";
import { RosterServiceClient } from "@tierklinik-dobersberg/angular/connect";
import { SaveRosterRequest, SaveRosterResponse } from "@tierklinik-dobersberg/apis";

export class SaveManager {
  constructor(
    private rosterService: RosterServiceClient,
  ) {}

  private casIndex: bigint | undefined = undefined;
  private inFlightPromise: Promise<void> = Promise.resolve();
  private inFlightTimeout: any | null = null;
  private saveIndex = 0;

  private readonly _inProgress = signal<boolean>(false);
  public readonly inProgress = this._inProgress.asReadonly();

  save(model: PartialMessage<SaveRosterRequest>, debounce?: number, force?: boolean): Promise<SaveRosterResponse> {
    // Clear the timeout if we're still debounced
    clearTimeout(this.inFlightTimeout);

    const saveIndex = (this.saveIndex++);

    const _save = () => new Promise<SaveRosterResponse>((resolve, reject) => {
      console.log(`[save-manager:#${saveIndex}] debouncing save`);

      this.inFlightTimeout = setTimeout(() => {
        // clear the timeout
        this.inFlightTimeout = null;

        this.inFlightPromise = this._performSave(model, saveIndex, force)
          .then(resolve)
          .catch(reject);

      }, debounce || 0);
    })

    return this.inFlightPromise.then(() => {
      return _save();
    })
  }

  setCasIndex(idx: bigint) {
    this.casIndex = idx;
  }

  private _performSave(model: PartialMessage<SaveRosterRequest>, idx: number, force = false): Promise<SaveRosterResponse> {
    if (force) {
      model.casIndex = undefined;
    } else {
      model.casIndex = this.casIndex;
    }

    this._inProgress.set(true);

    console.log(`[save-manager:#${idx}]: Saving roster with case-index`, this.casIndex)

    return this.rosterService
      .saveRoster(model)
      .then(response => {
        this.casIndex = response.roster!.casIndex;

        console.log(`[save-manager:#${idx}]: Roster saved successfully, new cas-index`, this.casIndex);

        return response;
      })
      .finally(() => {
        this._inProgress.set(false);
      })
  }
}
