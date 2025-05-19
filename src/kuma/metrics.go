package kuma

import (
	"errors"
	"log"
	"strconv"
	"strings"
)

const (
	MetricMonitorStatus = "monitor_status"
	LabelMonitorName    = "monitor_name"
	LabelMonitorType    = "monitor_type"
	LabelMonitorURL     = "monitor_url"
	LabelMonitorHost    = "monitor_hostname"
	LabelMonitorPort    = "monitor_port"
)

func shouldSkipLine(line string) bool {
	return line == "" || strings.HasPrefix(line, "#")
}

func extractMetricParts(line string) (metricName, labelStr, valueStr string, ok bool) {
	ok = false
	parts := strings.SplitN(line, "{", 2)
	if len(parts) != 2 {
		return
	}
	metricName = strings.TrimSpace(parts[0])

	labelsAndValue := strings.SplitN(parts[1], "}", 2)
	if len(labelsAndValue) != 2 {
		return
	}

	labelStr = labelsAndValue[0]
	valueStr = strings.TrimSpace(labelsAndValue[1])
	ok = true
	return
}

// parses the labels string into a service name and status
func parseLabels(labelStr string, statusValue int) (serviceName string, status ServiceStatus, err error) {
	if labelStr == "" {
		return "", ServiceStatus{}, errors.New("empty label string")
	}

	status.Status = statusValue
	labels := strings.Split(labelStr, ",")
	for _, label := range labels {
		kv := strings.SplitN(strings.TrimSpace(label), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := strings.Trim(kv[1], `"`)

		switch key {
		case LabelMonitorName:
			serviceName = value
		case LabelMonitorType:
			status.Type = value
		case LabelMonitorURL:
			status.URL = value
		case LabelMonitorHost:
			status.Hostname = value
		case LabelMonitorPort:
			status.Port = value
		}
	}

	if serviceName == "" {
		return "", ServiceStatus{}, errors.New("no monitor_name label found")
	}

	return serviceName, status, nil
}

func validateMetricName(name string) bool {
	return name == MetricMonitorStatus
}

// parses Prometheus-style metrics from Uptime Kuma
// and returns a map of service names to their status
func parseMetrics(data string) (map[string]ServiceStatus, error) {
	if data == "" {
		return nil, errors.New("empty metrics data")
	}

	metrics := make(map[string]ServiceStatus)

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if shouldSkipLine(line) {
			continue
		}

		name, labelStr, valueStr, ok := extractMetricParts(line)
		if !ok || !validateMetricName(name) {
			continue
		}

		statusValue, err := strconv.Atoi(valueStr)
		if err != nil {
			log.Printf("Failed to parse status value: %v", err)
			continue
		}

		serviceName, status, err := parseLabels(labelStr, statusValue)
		if err != nil {
			log.Printf("Failed to parse labels: %v", err)
			continue
		}

		metrics[serviceName] = status
	}

	if len(metrics) == 0 {
		return nil, errors.New("no valid metrics found")
	}

	return metrics, nil
}
