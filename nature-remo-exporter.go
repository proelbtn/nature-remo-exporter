package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version string

	cmd = kingpin.New("nature-remo-exporter", "Nature Remo Exporter")

	tags = []string{"id", "name", "serial_number"}
	temperature = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nature_remo_temperature",
		Help: "Temperature",
	}, tags)
	temperatureOffset = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nature_remo_temperature_offset",
		Help: "Temperature Offset",
	}, tags)
	humidity = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nature_remo_humidity",
		Help: "Humidity",
	}, tags)
	humidityOffset = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nature_remo_humidity_offset",
		Help: "Humidity Offset",
	}, tags)
	illumination = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "nature_remo_illumination",
		Help: "Illumination",
	}, tags)
)

type SensorValue struct {
	Val float64 `json:"val"`
	CreatedAt time.Time `json:"created_at"`
}

type Events struct {
	Temperature SensorValue `json:"te"`
	Humidity SensorValue `json:"hu"`
	Illumination SensorValue `json:"il"`
	Movement SensorValue `json:"mo"`
}

type Device struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	TemperatureOffset float64 `json:"temperature_offset"`
	HumidityOffset    float64 `json:"humidity_offset"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	FirmwareVersion   string    `json:"firmware_version"`
	MacAddress        string    `json:"mac_address"`
	SerialNumber      string    `json:"serial_number"`
	NewestEvents      Events `json:"newest_events"`
}

type NatureRemoConfig struct {
	ApiKey string `yaml:"api_key"`
	BaseUrl string `yaml:"base_url"`
}

type PromHttpConfig struct {
	ListenAddress string `yaml:"listen_address"`
}

type Config struct {
	NatureRemo NatureRemoConfig `yaml:"nature_remo"`
	PromHttp PromHttpConfig `yaml:"promhttp"`
}

func getLabel(device Device) prometheus.Labels {
	return prometheus.Labels{
		"id": device.ID,
		"name": device.Name,
		"serial_number": device.SerialNumber,
	}
}

func fetchDevices(config *Config) ([]Device, error) {
	var devices []Device

	path := fmt.Sprintf("https://%s/1/devices", config.NatureRemo.BaseUrl)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer " + config.NatureRemo.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	err = json.NewDecoder(resp.Body).Decode(&devices)
	return devices, err
}

func updateMetrics(config *Config) error {
	devices, err := fetchDevices(config)
	if err != nil {
		return nil
	}

	for _, device := range devices {
		labels := getLabel(device)
		temperature.With(labels).Set(device.NewestEvents.Temperature.Val)
		temperatureOffset.With(labels).Set(device.TemperatureOffset)
		humidity.With(labels).Set(device.NewestEvents.Humidity.Val)
		humidityOffset.With(labels).Set(device.TemperatureOffset)
		illumination.With(labels).Set(device.NewestEvents.Illumination.Val)

	}

	return nil
}

func poll(config *Config, cancel chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-ticker.C:
			go (func() {
				err := updateMetrics(config)
				if err != nil {
					cmd.Errorf("error while updating metrics: %s", err)
				}
			})()
		case <-cancel:
			break
		}
	}

}

func setup() (*Config, error) {
	var config string

	cmd.Author("proelbtn")
	cmd.Version(version)
	cmd.Flag("config", "Configuration file path").
		Default("config.yml").StringVar(&config)

	_, err := cmd.Parse(os.Args[1:])
	if err != nil {
		return nil, err
	}

	file, err := os.Open(config)
	if err != nil {
		return nil, errors.Errorf("couldn't open configuration file: %s", config)
	}

	cfg := &Config{}
	err = yaml.NewDecoder(file).Decode(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.NatureRemo.BaseUrl == "" {
		cfg.NatureRemo.BaseUrl = "api.nature.global"
	}

	return cfg, nil
}

func main() {
	cfg, err := setup()
	if err != nil {
		cmd.Errorf("error while setup: %s", err)
		os.Exit(1)
	}

	cancel := make(chan struct{})
	go poll(cfg, cancel)

	http.Handle("/metrics", promhttp.Handler())
	err = http.ListenAndServe(cfg.PromHttp.ListenAddress, nil)
	if err != nil {
		cmd.Errorf("error while serving promhttp: %s", err)
		os.Exit(1)
	}
}