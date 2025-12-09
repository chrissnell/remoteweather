-- Drop the manually-created alerts table so GORM AutoMigrate can manage it
DROP TABLE IF EXISTS aeris_weather_alerts CASCADE;
