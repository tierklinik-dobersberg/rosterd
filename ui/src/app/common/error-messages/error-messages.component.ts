import { JsonPipe } from "@angular/common";
import { AfterViewInit, ChangeDetectionStrategy, Component, ContentChild, DestroyRef, ElementRef, Renderer2, Signal, computed, inject, input, signal } from "@angular/core";
import { NgModel, ValidationErrors } from "@angular/forms";
import { hlm } from '@spartan-ng/ui-core';
import { HlmInputErrorDirective } from "@tierklinik-dobersberg/angular/input";
import { ClassArray, ClassValue, clsx } from 'clsx';
import { BehaviorSubject, Subscription } from 'rxjs';

const defaultErrorStyle = 'flex flex-row justify-start items-start';
const defaultStyle = 'w-full flex flex-col gap-1';
const knownErrors = ['durationFormat', 'required', 'min', 'max'];

interface MinError {
  min: number;
  actual: number;
}

interface MaxError {
  max: number;
  actual: number;
}

type RequiredError = boolean;
type StringError = string;

@Component({
  selector: 'app-error-messages',
  standalone: true,
  imports: [
    HlmInputErrorDirective,
    JsonPipe,
  ],
  // eslint-disable-next-line @angular-eslint/no-host-metadata-property
  host: {
    '[class]': '"flex flex-col w-full"'
  },
  templateUrl: './error-messages.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TkdErrorMessagesComponent implements AfterViewInit {
  @ContentChild(NgModel, { static: true })
  model!: NgModel;

  @ContentChild(NgModel, { static: true, read: ElementRef })
  modelElement!: ElementRef<HTMLElement>;

  readonly userClasses = input<ClassValue | ClassArray>([], { alias: 'class' })
  readonly errorClasses = input<ClassValue | ClassArray>([], { alias: 'errorClass' })

  private readonly renderer = inject(Renderer2);
  private readonly destroyRef = inject(DestroyRef);
  private statusSub = Subscription.EMPTY;

  constructor() {
    this.destroyRef
      .onDestroy(() => this.statusSub.unsubscribe());
  }

  ngAfterViewInit() {
    const model = this.model;

    this.statusSub.unsubscribe();

    if (!model || !model.statusChanges) {
      return
    }

    if (!this.modelElement) {
      return;
    }

    this.statusSub = new Subscription();

    const sub = new BehaviorSubject<void>(undefined);
    const cleanup = this.renderer.listen(this.modelElement.nativeElement, 'blur', () => {
      sub.next();
    })
    this.statusSub.add(cleanup);

    const update = () => {
      if (model.pristine && model.untouched) {
        this._errors.set(null);
      } else {
        this._errors.set(this.model?.errors || null);
      }
    };

    this.statusSub.add(
      sub.subscribe(() => update())
    );
    this.statusSub.add(
      this.model.statusChanges?.subscribe(() => update())
    )
  }

  protected readonly _computedClass = computed(() => {
    return hlm(clsx(defaultStyle, this.userClasses()));
  })

  protected readonly _computedErrorClass = computed(() => {
    return hlm(clsx(defaultErrorStyle, this.errorClasses()));
  })

  protected readonly _errors = signal<ValidationErrors | null>(null);

  protected readonly _durationFormat = this.getError<StringError>('durationFormat');
  protected readonly _required = this.getError<RequiredError>('required');
  protected readonly _min = this.getError<MinError>('min');
  protected readonly _max = this.getError<MaxError>('max')

  protected readonly _unknown = computed(() => {
    const errors = this._errors();
    if (!errors) {
      return null;
    }

    return Object.keys(errors)
      .filter(key => !knownErrors.includes(key))
      .map(key => ({
        key: key,
        error: errors[key],
      }));
  })

  private getError<T = unknown>(name: string): Signal<T | null> {
    return computed(() => {
      const errors = this._errors();
      if (!errors || !(name in errors)) {
        return null;
      }

      return errors[name];
    })
  }
}
