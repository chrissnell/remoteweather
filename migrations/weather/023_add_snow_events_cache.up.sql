-- Add snow events cache table for visualization
-- Events are computed every 15 minutes by the snow cache controller

CREATE TABLE IF NOT EXISTS snow_events_cache (
    stationname TEXT NOT NULL,
    hours INTEGER NOT NULL,           -- Time window (24, 72, 168 for 7d, etc.)
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,         -- 'accumulation', 'plateau', 'redistribution', 'spike_then_settle'
    start_depth_mm REAL NOT NULL,
    end_depth_mm REAL NOT NULL,
    accumulation_mm REAL,             -- Only for accumulation/spike events
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (stationname, hours, start_time)
);

-- Index for efficient querying by station and hours
CREATE INDEX IF NOT EXISTS idx_snow_events_station_hours
    ON snow_events_cache(stationname, hours);

-- Index for computed_at to efficiently find fresh cache entries
CREATE INDEX IF NOT EXISTS idx_snow_events_computed
    ON snow_events_cache(stationname, hours, computed_at DESC);

COMMENT ON TABLE snow_events_cache IS
'Cached snow events for visualization. Updated every 15 minutes by snow cache controller.
Events are detected using PELT change point detection algorithm.';
