package provider

import (
	"regexp"
	"strconv"
	"strings"
)

var priceRegex = regexp.MustCompile(`\$?\s*(\d+(?:\.\d{1,2})?)`)

// parsePrice extracts a numeric price from a string like "$4.99", "4.99/lb", "2/$5.00".
// Returns 0 if no valid price is found.
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Handle "2/$5.00" format — return per-unit price
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		if len(parts) == 2 {
			qty, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			if err == nil && qty > 0 {
				match := priceRegex.FindStringSubmatch(parts[1])
				if len(match) > 1 {
					price, err := strconv.ParseFloat(match[1], 64)
					if err == nil {
						return price / qty
					}
				}
			}
		}
	}

	// Standard price extraction
	match := priceRegex.FindStringSubmatch(s)
	if len(match) > 1 {
		price, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			return price
		}
	}

	return 0
}
