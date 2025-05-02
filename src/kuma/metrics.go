package kuma

import (
	"strconv"
	"strings"
)

func parseMetrics(data string) (map[string]ServiceStatus, error) {
	metrics := make(map[string]ServiceStatus)
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract the metric name and labels
		parts := strings.SplitN(line, "{", 2)
		if len(parts) != 2 {
			continue
		}

		// Skip if not monitor_status metric
		if strings.TrimSpace(parts[0]) != "monitor_status" {
			continue
		}

		// Extract labels and value
		labelsAndValue := strings.SplitN(parts[1], "}", 2)
		if len(labelsAndValue) != 2 {
			continue
		}

		labelsPart := labelsAndValue[0]
		valuePart := strings.TrimSpace(labelsAndValue[1])

		// Parse the status value
		statusValue, err := strconv.Atoi(valuePart)
		if err != nil {
			continue
		}

		// Extract labels
		labels := strings.Split(labelsPart, ",")
		var serviceName string
		var status ServiceStatus
		status.Status = statusValue

		for _, label := range labels {
			label = strings.TrimSpace(label)
			keyValue := strings.SplitN(label, "=", 2)
			if len(keyValue) != 2 {
				continue
			}

			key := strings.TrimSpace(keyValue[0])
			value := strings.Trim(strings.TrimSpace(keyValue[1]), "\"")

			switch key {
			case "monitor_name":
				serviceName = value
			case "monitor_type":
				status.Type = value
			case "monitor_url":
				status.URL = value
			case "monitor_hostname":
				status.Hostname = value
			case "monitor_port":
				status.Port = value
			}
		}

		if serviceName == "" {
			continue
		}

		metrics[serviceName] = status
	}

	return metrics, nil
}
