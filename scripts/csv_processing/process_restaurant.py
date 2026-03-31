from typing import List

import pandas as pd

from .service_seed_common import build_restaurants as build_restaurant_records


def build_restaurant_data(places_df: pd.DataFrame, reviews_df: pd.DataFrame, tripadvisor_df: pd.DataFrame) -> List:
    """Build restaurant records from places, reviews, and tripadvisor data."""
    return build_restaurant_records(
        places_df=places_df,
        reviews_df=reviews_df,
        tripadvisor_df=tripadvisor_df,
        max_restaurants=None,
    )
