-- Remove the inefficient today_rainfall view that was replaced by optimized Go code
-- in commit 45a3e98. The view was causing performance issues (778ms queries) and 
-- wasn't station-specific. Daily rainfall is now calculated using the optimized
-- CalculateDailyRainfall() function in Go code.
DROP VIEW IF EXISTS today_rainfall;