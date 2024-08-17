import { AnySchema } from "../../schema/schema";
import { InferSchemaData, InferSchemaOutput } from "../../schema/type-helpers";
import { WithOptionalId } from "../../type-helpers";
import { BaseInsertManyQuery } from "./base";

export class InsertManyQuery<
    T extends AnySchema
> extends BaseInsertManyQuery<T> {
    async exec(): Promise<InferSchemaOutput<T>[]> {
        const result = await this._collection.insertMany(
            this.data as WithOptionalId<InferSchemaData<T>>[]
        );
        return this.data.map((data, index) =>
            this._schema.fromData({ _id: result.insertedIds[index], ...data })
        );
    }
}