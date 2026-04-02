# Scripts ID Strategy

The source `gPlus...` identifiers are 22 characters long, which is too large for the `BIGINT` columns used by the seed databases. We still need the generated IDs to be stable, deterministic, and consistent across runs, so the same source value must always produce the same output value.

We first considered a min/max range remapping approach, but it still did not give us a simple fixed-width ID strategy for every source value. In the end, we switched to hashing the source IDs into a positive `BIGINT` space.

This gives us a deterministic transformation: the same input always produces the same output.

The hash space is about $1.84 \times 10^{19}$ possible values. With roughly 3.5 million restaurant entries in the dataset, the collision probability is extremely small, about $3.3 \times 10^{-7}$. That makes hashing a practical tradeoff for keeping IDs compact while preserving consistency.
