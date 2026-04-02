# Scripts ID Strategy

The source `gPlus...` identifiers are 22 characters long, which is too large for the `BIGINT` columns used by the seed databases. We still need the generated IDs to be stable, deterministic, and consistent across runs, so the same source value must always produce the same output value.

We first considered a min/max range remapping approach, but it still did not give us a simple fixed-width ID strategy for every source value. In the end, we switched to hashing the source IDs into a positive `BIGINT` space.

This gives us a deterministic transformation: the same input always produces the same output.

The hash space is about $1.84 \times 10^{19}$ possible values. With roughly 3.5 million restaurant entries in the dataset, the collision probability is extremely small, about $3.3 \times 10^{-7}$. That makes hashing a practical tradeoff for keeping IDs compact while preserving consistency.

## Process

The main entry point is [process_data.py](process_data.py). It reads the raw CSV files from [../data](../data), validates that the required source files exist for the selected services, and then generates the processed seed files under [processed_data](processed_data).

The generation flow is service-aware:

1. `auth` processes user data and writes `app_user.csv`.
2. `restaurant` builds restaurant records from `places.csv` and `tripadvisor_european_restaurants.csv`, then writes the restaurant to the CSVs.
3. `review` reuses the generated user and restaurant mappings to build review records and writes `review.csv`.

By default, the script processes a limited number of rows for quicker runs. Use `--rows` to change that limit or `--full` to process the full dataset.

## Structure

- [process_data.py](process_data.py) contains the command-line entry point and overall orchestration.
- [csv_processing](csv_processing) contains the service-specific transformation logic.
- [processed_data](processed_data) stores the generated output files.
- [../data](../data) contains the source datasets consumed by the pipeline.

The structure is intentionally split so the orchestration code stays small while each service keeps its own CSV transformation rules in a dedicated module.
