-- +migrate up

CREATE TABLE weather (
    record TIMESTAMP PRIMARY KEY,
    temperature FLOAT NOT NULL,
    pressure FLOAT NOT NULL,
    rain_mm FLOAT NOT NULL,
    wind_speed FLOAT NOT NULL,
    wind_direction INT NOT NULL
);