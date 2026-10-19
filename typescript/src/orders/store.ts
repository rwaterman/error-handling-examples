export type StoredRecord = Readonly<Record<string, unknown>>;

export interface OrderStore {
  get(key: string): Promise<StoredRecord | undefined>;
  put(key: string, value: StoredRecord): Promise<void>;
}

export class InMemoryOrderStore implements OrderStore {
  readonly records = new Map<string, StoredRecord>();

  get(key: string): Promise<StoredRecord | undefined> {
    return Promise.resolve(this.records.get(key));
  }

  put(key: string, value: StoredRecord): Promise<void> {
    this.records.set(key, value);
    return Promise.resolve();
  }
}
