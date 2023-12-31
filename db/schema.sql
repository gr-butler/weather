-- +migrate up

CREATE TABLE weather (
    record_date TIMESTAMP without timezone PRIMARY KEY,
    temperature FLOAT NOT NULL,
    pressure FLOAT NOT NULL,
    rain_mm FLOAT NOT NULL,
    wind_speed FLOAT NOT NULL,
    wind_gust FLOAT NOT NULL,
    wind_direction FLOAT NOT NULL
);