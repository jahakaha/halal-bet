package util

var teamFlags = map[string]string{
	// South America
	"Argentina": "🇦🇷",
	"Brazil":    "🇧🇷",
	"Uruguay":   "🇺🇾",
	"Colombia":  "🇨🇴",
	"Ecuador":   "🇪🇨",
	"Paraguay":  "🇵🇾",
	"Venezuela": "🇻🇪",
	"Bolivia":   "🇧🇴",
	"Peru":      "🇵🇪",
	"Chile":     "🇨🇱",

	// Europe
	"Germany":        "🇩🇪",
	"France":         "🇫🇷",
	"Spain":          "🇪🇸",
	"England":        "🏴󠁧󠁢󠁥󠁮󠁧󠁿",
	"Portugal":       "🇵🇹",
	"Netherlands":    "🇳🇱",
	"Belgium":        "🇧🇪",
	"Croatia":        "🇭🇷",
	"Serbia":         "🇷🇸",
	"Denmark":        "🇩🇰",
	"Austria":        "🇦🇹",
	"Switzerland":    "🇨🇭",
	"Scotland":       "🏴󠁧󠁢󠁳󠁣󠁴󠁿",
	"Poland":         "🇵🇱",
	"Turkey":         "🇹🇷",
	"Ukraine":        "🇺🇦",
	"Hungary":        "🇭🇺",
	"Slovakia":       "🇸🇰",
	"Romania":        "🇷🇴",
	"Czech Republic": "🇨🇿",
	"Czechia":        "🇨🇿",
	"Slovenia":       "🇸🇮",
	"Albania":        "🇦🇱",
	"Georgia":        "🇬🇪",
	"Italy":          "🇮🇹",
	"Greece":         "🇬🇷",
	"Norway":         "🇳🇴",
	"Sweden":         "🇸🇪",
	"Finland":        "🇫🇮",
	"Iceland":        "🇮🇸",
	"Wales":          "🏴󠁧󠁢󠁷󠁬󠁳󠁿",
	"Ireland":        "🇮🇪",

	// North/Central America & Caribbean
	"United States":       "🇺🇸",
	"USA":                 "🇺🇸",
	"Mexico":              "🇲🇽",
	"Canada":              "🇨🇦",
	"Panama":              "🇵🇦",
	"Jamaica":             "🇯🇲",
	"Honduras":            "🇭🇳",
	"El Salvador":         "🇸🇻",
	"Costa Rica":          "🇨🇷",
	"Trinidad and Tobago": "🇹🇹",
	"Guatemala":           "🇬🇹",
	"Cuba":                "🇨🇺",

	// Africa
	"Morocco":      "🇲🇦",
	"Senegal":      "🇸🇳",
	"Nigeria":      "🇳🇬",
	"Ghana":        "🇬🇭",
	"Cameroon":     "🇨🇲",
	"Egypt":        "🇪🇬",
	"Algeria":      "🇩🇿",
	"Tunisia":      "🇹🇳",
	"Mali":         "🇲🇱",
	"South Africa": "🇿🇦",
	"Ivory Coast":  "🇨🇮",
	"DR Congo":     "🇨🇩",
	"Congo":        "🇨🇬",
	"Tanzania":     "🇹🇿",
	"Uganda":       "🇺🇬",
	"Zambia":       "🇿🇲",

	// Asia
	"Japan":          "🇯🇵",
	"South Korea":    "🇰🇷",
	"Korea Republic": "🇰🇷",
	"Australia":      "🇦🇺",
	"Iran":           "🇮🇷",
	"Saudi Arabia":   "🇸🇦",
	"Qatar":          "🇶🇦",
	"Iraq":           "🇮🇶",
	"Jordan":         "🇯🇴",
	"Uzbekistan":     "🇺🇿",
	"Oman":           "🇴🇲",
	"China PR":       "🇨🇳",
	"China":          "🇨🇳",
	"UAE":            "🇦🇪",
	"Bahrain":        "🇧🇭",
	"Kuwait":         "🇰🇼",

	// Oceania
	"New Zealand": "🇳🇿",
}

func WithFlag(name string) string {
	if flag, ok := teamFlags[name]; ok {
		return flag + " " + name
	}
	return name
}
