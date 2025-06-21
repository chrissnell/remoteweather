package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"

	"github.com/chrissnell/remoteweather/pkg/config"
)

func main() {
	var (
		yamlFile   = flag.String("yaml", "", "Path to YAML configuration file")
		sqliteFile = flag.String("sqlite", "", "Path to SQLite configuration file")
	)
	flag.Parse()

	if *yamlFile == "" || *sqliteFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -yaml <config.yaml> -sqlite <config.db>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Println("Configuration Comparison Test")
	fmt.Println("===========================")

	// Load YAML configuration
	fmt.Printf("Loading YAML configuration: %s\n", *yamlFile)
	yamlProvider := config.NewYAMLProvider(*yamlFile)
	yamlConfig, err := yamlProvider.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading YAML config: %v\n", err)
		os.Exit(1)
	}

	// Load SQLite configuration
	fmt.Printf("Loading SQLite configuration: %s\n", *sqliteFile)
	sqliteProvider, err := config.NewSQLiteProvider(*sqliteFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating SQLite provider: %v\n", err)
		os.Exit(1)
	}
	defer sqliteProvider.Close()

	sqliteConfig, err := sqliteProvider.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading SQLite config: %v\n", err)
		os.Exit(1)
	}

	// Compare configurations
	fmt.Println("\nComparison Results:")
	fmt.Println("==================")

	// Compare devices
	fmt.Printf("Devices - YAML: %d, SQLite: %d\n", len(yamlConfig.Devices), len(sqliteConfig.Devices))
	if len(yamlConfig.Devices) == len(sqliteConfig.Devices) {
		fmt.Println("✓ Device count matches")
		for i, yamlDevice := range yamlConfig.Devices {
			if i < len(sqliteConfig.Devices) {
				sqliteDevice := sqliteConfig.Devices[i]
				if compareDevices(yamlDevice, sqliteDevice) {
					fmt.Printf("✓ Device %s matches\n", yamlDevice.Name)
				} else {
					fmt.Printf("✗ Device %s differs\n", yamlDevice.Name)
					printDeviceDiff(yamlDevice, sqliteDevice)
				}
			}
		}
	} else {
		fmt.Println("✗ Device count mismatch")
	}

	// Compare storage
	fmt.Println("\nStorage Configuration:")
	compareStorage(yamlConfig.Storage, sqliteConfig.Storage)

	// Compare controllers
	fmt.Printf("\nControllers - YAML: %d, SQLite: %d\n", len(yamlConfig.Controllers), len(sqliteConfig.Controllers))
	if len(yamlConfig.Controllers) == len(sqliteConfig.Controllers) {
		fmt.Println("✓ Controller count matches")
		for i, yamlController := range yamlConfig.Controllers {
			if i < len(sqliteConfig.Controllers) {
				sqliteController := sqliteConfig.Controllers[i]
				if compareControllers(yamlController, sqliteController) {
					fmt.Printf("✓ Controller %s matches\n", yamlController.Type)
				} else {
					fmt.Printf("✗ Controller %s differs\n", yamlController.Type)
				}
			}
		}
	} else {
		fmt.Println("✗ Controller count mismatch")
	}

	fmt.Println("\nTest completed!")
}

func compareDevices(yaml, sqlite config.DeviceData) bool {
	return yaml.Name == sqlite.Name &&
		yaml.Type == sqlite.Type &&
		yaml.Hostname == sqlite.Hostname &&
		yaml.Port == sqlite.Port &&
		yaml.SerialDevice == sqlite.SerialDevice &&
		yaml.Baud == sqlite.Baud &&
		yaml.WindDirCorrection == sqlite.WindDirCorrection &&
		yaml.BaseSnowDistance == sqlite.BaseSnowDistance &&
		compareSolar(yaml.Solar, sqlite.Solar)
}

func compareSolar(yaml, sqlite config.SolarData) bool {
	tolerance := 0.000001
	return abs(yaml.Latitude-sqlite.Latitude) < tolerance &&
		abs(yaml.Longitude-sqlite.Longitude) < tolerance &&
		abs(yaml.Altitude-sqlite.Altitude) < tolerance
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func printDeviceDiff(yaml, sqlite config.DeviceData) {
	if yaml.Name != sqlite.Name {
		fmt.Printf("  Name: YAML='%s', SQLite='%s'\n", yaml.Name, sqlite.Name)
	}
	if yaml.Type != sqlite.Type {
		fmt.Printf("  Type: YAML='%s', SQLite='%s'\n", yaml.Type, sqlite.Type)
	}
	if yaml.Hostname != sqlite.Hostname {
		fmt.Printf("  Hostname: YAML='%s', SQLite='%s'\n", yaml.Hostname, sqlite.Hostname)
	}
	if yaml.Port != sqlite.Port {
		fmt.Printf("  Port: YAML='%s', SQLite='%s'\n", yaml.Port, sqlite.Port)
	}
}

func compareStorage(yaml, sqlite config.StorageData) {
	// Compare InfluxDB
	if (yaml.InfluxDB == nil) != (sqlite.InfluxDB == nil) {
		fmt.Println("✗ InfluxDB configuration presence mismatch")
	} else if yaml.InfluxDB != nil && sqlite.InfluxDB != nil {
		if reflect.DeepEqual(*yaml.InfluxDB, *sqlite.InfluxDB) {
			fmt.Println("✓ InfluxDB configuration matches")
		} else {
			fmt.Println("✗ InfluxDB configuration differs")
		}
	} else {
		fmt.Println("✓ InfluxDB: both nil")
	}

	// Compare TimescaleDB
	if (yaml.TimescaleDB == nil) != (sqlite.TimescaleDB == nil) {
		fmt.Println("✗ TimescaleDB configuration presence mismatch")
	} else if yaml.TimescaleDB != nil && sqlite.TimescaleDB != nil {
		if reflect.DeepEqual(*yaml.TimescaleDB, *sqlite.TimescaleDB) {
			fmt.Println("✓ TimescaleDB configuration matches")
		} else {
			fmt.Println("✗ TimescaleDB configuration differs")
		}
	} else {
		fmt.Println("✓ TimescaleDB: both nil")
	}

	// Compare GRPC
	if (yaml.GRPC == nil) != (sqlite.GRPC == nil) {
		fmt.Println("✗ GRPC configuration presence mismatch")
	} else if yaml.GRPC != nil && sqlite.GRPC != nil {
		if reflect.DeepEqual(*yaml.GRPC, *sqlite.GRPC) {
			fmt.Println("✓ GRPC configuration matches")
		} else {
			fmt.Println("✗ GRPC configuration differs")
		}
	} else {
		fmt.Println("✓ GRPC: both nil")
	}

	// Compare APRS
	if (yaml.APRS == nil) != (sqlite.APRS == nil) {
		fmt.Println("✗ APRS configuration presence mismatch")
	} else if yaml.APRS != nil && sqlite.APRS != nil {
		if reflect.DeepEqual(*yaml.APRS, *sqlite.APRS) {
			fmt.Println("✓ APRS configuration matches")
		} else {
			fmt.Println("✗ APRS configuration differs")
		}
	} else {
		fmt.Println("✓ APRS: both nil")
	}
}

func compareControllers(yaml, sqlite config.ControllerData) bool {
	if yaml.Type != sqlite.Type {
		return false
	}

	// Compare each controller type
	if (yaml.PWSWeather == nil) != (sqlite.PWSWeather == nil) {
		return false
	}
	if yaml.PWSWeather != nil && !reflect.DeepEqual(*yaml.PWSWeather, *sqlite.PWSWeather) {
		return false
	}

	if (yaml.WeatherUnderground == nil) != (sqlite.WeatherUnderground == nil) {
		return false
	}
	if yaml.WeatherUnderground != nil && !reflect.DeepEqual(*yaml.WeatherUnderground, *sqlite.WeatherUnderground) {
		return false
	}

	if (yaml.AerisWeather == nil) != (sqlite.AerisWeather == nil) {
		return false
	}
	if yaml.AerisWeather != nil && !reflect.DeepEqual(*yaml.AerisWeather, *sqlite.AerisWeather) {
		return false
	}

	if (yaml.RESTServer == nil) != (sqlite.RESTServer == nil) {
		return false
	}
	if yaml.RESTServer != nil && !reflect.DeepEqual(*yaml.RESTServer, *sqlite.RESTServer) {
		return false
	}

	return true
}
