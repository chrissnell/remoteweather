-- Add sun_times table for pre-calculated sunrise/sunset times per station
CREATE TABLE IF NOT EXISTS sun_times (
    station_name TEXT NOT NULL,
    day_of_year INTEGER NOT NULL CHECK (day_of_year >= 1 AND day_of_year <= 366),
    sunrise_minutes INTEGER NOT NULL,
    sunset_minutes INTEGER NOT NULL,
    PRIMARY KEY (station_name, day_of_year)
);
