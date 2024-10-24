import type {
  Filter,
  FindOptions,
  Collection as MongoCollection,
} from "mongodb";
import type { SchemaRelationSelect } from "../../schema/refs";
import { type AnySchema, Schema } from "../../schema/schema";
import type {
  InferSchemaData,
  InferSchemaOmit,
  InferSchemaOutput,
} from "../../schema/type-helpers";
import type { Pretty, TrueKeys } from "../../type-helpers";
import type { PipelineStage } from "../types/pipeline-stage";
import type {
  BoolProjection,
  Projection,
  WithProjection,
} from "../types/query-options";
import {
  addExtraInputsToProjection,
  makeProjection,
} from "../utils/projection";
import { Query } from "./base";

export class FindOneQuery<
  T extends AnySchema,
  O = WithProjection<"omit", InferSchemaOutput<T>, InferSchemaOmit<T>>,
> extends Query<T, O | null> {
  private _projection: Projection<InferSchemaOutput<T>>;
  private _population: SchemaRelationSelect<T> = {};

  constructor(
    protected _schema: T,
    protected _collection: MongoCollection<InferSchemaData<T>>,
    protected _readyPromise: Promise<void>,
    private _filter: Filter<InferSchemaData<T>>,
    private _options: FindOptions = {},
  ) {
    super(_schema, _collection, _readyPromise);
    this._projection = makeProjection("omit", _schema.options.omit ?? {});
  }

  public options(options: FindOptions): this {
    Object.assign(this._options, options);
    return this;
  }

  public omit<P extends BoolProjection<InferSchemaOutput<T>>>(projection: P) {
    this._projection = makeProjection("omit", projection);
    return this as FindOneQuery<
      T,
      WithProjection<"omit", InferSchemaOutput<T>, TrueKeys<P>>
    >;
  }

  public select<P extends BoolProjection<InferSchemaOutput<T>>>(projection: P) {
    this._projection = makeProjection("select", projection);
    return this as FindOneQuery<
      T,
      WithProjection<"select", InferSchemaOutput<T>, TrueKeys<P>>
    >;
  }

  public populate<P extends Pretty<SchemaRelationSelect<T>>>(population: P) {
    Object.assign(this._population, population);
    return this as FindOneQuery<T, InferSchemaOutput<T>>;
  }

  public async exec(): Promise<O | null> {
    await this._readyPromise;
    if (Object.keys(this._population).length) {
      return this._execWithPopulate();
    }
    return this._execWithoutPopulate();
  }

  private async _execWithoutPopulate(): Promise<O | null> {
    const extra = addExtraInputsToProjection(
      this._projection,
      this._schema.options.virtuals,
    );
    const res = await this._collection.findOne(this._filter, {
      ...this._options,
      projection: this._projection,
    });
    return res
      ? (Schema.fromData(
          this._schema,
          res as InferSchemaData<T>,
          this._projection,
          extra,
        ) as O)
      : res;
  }

  private async _execWithPopulate(): Promise<O | null> {
    const pipeline: PipelineStage<InferSchemaOutput<T>>[] = [
      // @ts-expect-error
      { $match: this._filter },
    ];
    for (const [key, value] of Object.entries(this._population)) {
      if (!value) continue;
      const population = this._schema.relations[key];
      const foreignCollectionName = population.target.name;
      const foreignField = population.options.field;
      const foreignFieldVariable = `monarch_${foreignField}_var`;
      const foreignFieldData = `monarch_${key}_data`;
      pipeline.push({
        $lookup: {
          from: foreignCollectionName,
          let: { [foreignFieldVariable]: `$${key}` },
          pipeline: [
            {
              // @ts-expect-error
              $match: {
                $expr: {
                  $eq: [`$${foreignField}`, `$$${foreignFieldVariable}`],
                },
              },
            },
          ],
          as: foreignFieldData,
        },
      });
      pipeline.push({ $unwind: `$${foreignFieldData}` }); // Unwind the populated field if it's an array
      pipeline.push({
        $unset: key,
      });
      pipeline.push({
        $set: {
          [key]: `$${foreignFieldData}`,
        },
      });
      pipeline.push({
        $unset: foreignFieldData,
      });
    }
    const result = await this._collection.aggregate(pipeline).toArray();
    return result.length > 0
      ? (Schema.fromData(
          this._schema,
          result[0] as InferSchemaData<T>,
          this._projection,
          null,
        ) as O)
      : null;
  }
}
