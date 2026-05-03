from __future__ import annotations

import os
import random

from pymilvus import Collection, CollectionSchema, DataType, FieldSchema, connections, utility


def main() -> None:
    host = os.getenv("MILVUS_HOST", "milvus")
    port = os.getenv("MILVUS_PORT", "19530")
    collection_name = os.getenv("MILVUS_COLLECTION", "forge_phase0_smoke")
    dimension = 8

    connections.connect(alias="default", host=host, port=port)
    if utility.has_collection(collection_name):
        utility.drop_collection(collection_name)

    schema = CollectionSchema(
        fields=[
            FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=False),
            FieldSchema(name="tenant_id", dtype=DataType.VARCHAR, max_length=64),
            FieldSchema(name="workspace_id", dtype=DataType.VARCHAR, max_length=64),
            FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=dimension),
        ],
        description="Forge Phase 0 synthetic RAG validation collection",
    )
    collection = Collection(collection_name, schema=schema)

    random.seed(7)
    ids = list(range(1, 6))
    vectors = [[float(i)] + [random.random() for _ in range(dimension - 1)] for i in ids]
    collection.insert([ids, ["tenant-local"] * 5, ["workspace-local"] * 5, vectors])
    collection.flush()
    collection.create_index("embedding", {"index_type": "HNSW", "metric_type": "L2", "params": {"M": 8, "efConstruction": 64}})
    collection.load()

    results = collection.search(
        data=[vectors[2]],
        anns_field="embedding",
        param={"metric_type": "L2", "params": {"ef": 16}},
        limit=1,
        output_fields=["tenant_id", "workspace_id"],
    )
    top = results[0][0]
    if top.id != ids[2]:
        raise SystemExit(f"expected nearest id {ids[2]}, got {top.id}")
    print(f"milvus smoke ok: collection={collection_name} nearest_id={top.id}")


if __name__ == "__main__":
    main()
