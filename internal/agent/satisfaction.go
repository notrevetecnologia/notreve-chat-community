package agent

import (
	"strconv"
	"strings"
	"unicode"
)

func ParseSatisfactionRating(input string) (int, bool) {
	text := strings.TrimSpace(strings.ToLower(input))
	if text == "" {
		return 0, false
	}

	trimmed := strings.Trim(text, " \t\r\n.,!?;:")
	labels := map[string]int{
		"péssimo":   1,
		"pessimo":   1,
		"ruim":      2,
		"regular":   3,
		"bom":       4,
		"muito bom": 5,
		"ótimo":     5,
		"otimo":     5,
		"excelente": 5,
	}
	if rating, ok := labels[trimmed]; ok {
		return rating, true
	}

	stars := 0
	starsOnly := true
	for _, r := range text {
		switch {
		case r == '★' || r == '⭐':
			stars++
		case unicode.IsSpace(r) || unicode.IsPunct(r) || r == '\ufe0f':
		default:
			starsOnly = false
		}
	}
	if starsOnly && stars >= 1 && stars <= 5 {
		return stars, true
	}

	var numbers []int
	for i := 0; i < len(text); {
		if text[i] < '0' || text[i] > '9' {
			i++
			continue
		}
		start := i
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
		}
		n, err := strconv.Atoi(text[start:i])
		if err != nil {
			return 0, false
		}
		numbers = append(numbers, n)
	}
	if len(numbers) == 1 && numbers[0] >= 1 && numbers[0] <= 5 {
		return numbers[0], true
	}
	if len(numbers) == 2 && numbers[0] >= 1 && numbers[0] <= 5 && numbers[1] == 5 && strings.Contains(text, "/") {
		return numbers[0], true
	}
	return 0, false
}
