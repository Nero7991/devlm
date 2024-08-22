package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// IsValidEmail checks if the given email address is valid using a comprehensive regular expression.
func IsValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]
	_, err := net.LookupMX(domain)
	return err == nil
}

// IsValidURL checks if the given URL is valid and includes additional validation rules.
func IsValidURL(rawURL string) bool {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return false
	}
	switch parsedURL.Scheme {
	case "http", "https", "ftp", "sftp", "mailto", "file", "data":
		return true
	default:
		return false
	}
}

// IsAlphanumeric checks if the given string contains only alphanumeric characters.
func IsAlphanumeric(s string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(s)
}

// IsNumeric checks if the given string contains only numeric characters, including decimal numbers.
func IsNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false
	}
	// Check for scientific notation
	if strings.ContainsAny(s, "eE") {
		parts := strings.FieldsFunc(s, func(r rune) bool {
			return r == 'e' || r == 'E'
		})
		if len(parts) != 2 {
			return false
		}
		_, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return false
		}
		_, err = strconv.Atoi(parts[1])
		return err == nil
	}
	return true
}

// IsValidIPv4 checks if the given string is a valid IPv4 address.
func IsValidIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() != nil
}

// IsValidIPv6 checks if the given string is a valid IPv6 address.
func IsValidIPv6(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() == nil && parsedIP.To16() != nil
}

// IsValidPassword checks if the given password meets the specified criteria with customizable requirements.
func IsValidPassword(password string, minLength, maxLength int, requireUpper, requireLower, requireNumber, requireSpecial bool, customCharSet string) bool {
	if len(password) < minLength || len(password) > maxLength {
		return false
	}
	var hasUpper, hasLower, hasNumber, hasSpecial, hasCustom bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		case strings.ContainsRune(customCharSet, char):
			hasCustom = true
		}
	}
	return (!requireUpper || hasUpper) &&
		(!requireLower || hasLower) &&
		(!requireNumber || hasNumber) &&
		(!requireSpecial || hasSpecial) &&
		(customCharSet == "" || hasCustom)
}

// IsValidUsername checks if the given username is valid based on customizable rules.
func IsValidUsername(username string, minLength, maxLength int, allowedChars string, reservedUsernames []string) bool {
	if len(username) < minLength || len(username) > maxLength {
		return false
	}
	validChars := regexp.MustCompile(fmt.Sprintf("^[%s]+$", regexp.QuoteMeta(allowedChars)))
	if !validChars.MatchString(username) {
		return false
	}
	for _, reserved := range reservedUsernames {
		if strings.EqualFold(username, reserved) {
			return false
		}
	}
	return true
}

// IsEmptyOrWhitespace checks if the given string is empty or contains only whitespace.
func IsEmptyOrWhitespace(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// IsValidPhoneNumber checks if the given phone number is valid for the specified country code.
func IsValidPhoneNumber(phoneNumber string, countryCode string) bool {
	phoneRegex := map[string]string{
		"US": `^\+1[2-9]\d{2}[2-9]\d{2}\d{4}$`,
		"UK": `^\+44[1-9]\d{9}$`,
		"IN": `^\+91[6-9]\d{9}$`,
		"AU": `^\+61[1-9]\d{8}$`,
		"CA": `^\+1[2-9]\d{2}[2-9]\d{2}\d{4}$`,
		"DE": `^\+49[1-9]\d{10}$`,
		"FR": `^\+33[1-9]\d{8}$`,
		"JP": `^\+81[1-9]\d{9}$`,
	}
	pattern, exists := phoneRegex[countryCode]
	if !exists {
		pattern = `^\+?[1-9]\d{1,14}$` // Default international format
	}
	return regexp.MustCompile(pattern).MatchString(phoneNumber)
}

// IsValidCreditCardNumber checks if the given credit card number is valid using the Luhn algorithm.
func IsValidCreditCardNumber(cardNumber string) bool {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, cardNumber)

	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	isEven := false

	for i := len(digits) - 1; i >= 0; i-- {
		digit := int(digits[i] - '0')
		if isEven {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		isEven = !isEven
	}

	return sum%10 == 0
}

// GetCreditCardType returns the type of credit card based on its number.
func GetCreditCardType(cardNumber string) string {
	patterns := map[string]string{
		"Visa":       `^4[0-9]{12}(?:[0-9]{3})?$`,
		"MasterCard": `^(5[1-5][0-9]{14}|2[2-7][0-9]{14})$`,
		"Amex":       `^3[47][0-9]{13}$`,
		"Discover":   `^6(?:011|5[0-9]{2})[0-9]{12}$`,
		"JCB":        `^(?:2131|1800|35\d{3})\d{11}$`,
		"DinersClub": `^3(?:0[0-5]|[68][0-9])[0-9]{11}$`,
		"UnionPay":   `^(62[0-9]{14,17})$`,
		"Maestro":    `^(5018|5020|5038|6304|6759|6761|6763)[0-9]{8,15}$`,
	}

	for cardType, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(cardNumber) {
			return cardType
		}
	}
	return "Unknown"
}

// IsValidPostalCode checks if the given postal code is valid for the specified country.
func IsValidPostalCode(postalCode string, countryCode string) bool {
	postalCodeRegex := map[string]string{
		"US": `^\d{5}(-\d{4})?$`,
		"UK": `^[A-Z]{1,2}[0-9][A-Z0-9]? [0-9][ABD-HJLNP-UW-Z]{2}$`,
		"CA": `^[ABCEGHJKLMNPRSTVXY]\d[ABCEGHJ-NPRSTV-Z][ ]?\d[ABCEGHJ-NPRSTV-Z]\d$`,
		"AU": `^\d{4}$`,
		"IN": `^\d{6}$`,
		"DE": `^\d{5}$`,
		"FR": `^\d{5}$`,
		"JP": `^\d{3}-\d{4}$`,
	}
	pattern, exists := postalCodeRegex[countryCode]
	if !exists {
		return true // If no specific pattern is defined, assume it's valid
	}
	return regexp.MustCompile(pattern).MatchString(postalCode)
}

// IsValidISBN checks if the given string is a valid ISBN (10 or 13 digits).
func IsValidISBN(isbn string) bool {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")

	if len(isbn) == 10 {
		return isValidISBN10(isbn)
	} else if len(isbn) == 13 {
		return isValidISBN13(isbn)
	}
	return false
}

func isValidISBN10(isbn string) bool {
	sum := 0
	for i := 0; i < 9; i++ {
		digit := int(isbn[i] - '0')
		sum += digit * (10 - i)
	}
	lastChar := isbn[9]
	if lastChar == 'X' {
		sum += 10
	} else {
		sum += int(lastChar - '0')
	}
	return sum%11 == 0
}

func isValidISBN13(isbn string) bool {
	sum := 0
	for i := 0; i < 12; i++ {
		digit := int(isbn[i] - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += 3 * digit
		}
	}
	checkDigit := (10 - (sum % 10)) % 10
	return checkDigit == int(isbn[12]-'0')
}

// IsValidHexColor checks if the given string is a valid hexadecimal color code.
func IsValidHexColor(color string) bool {
	match, _ := regexp.MatchString(`^#(?:[0-9a-fA-F]{3}){1,2}$`, color)
	return match
}

// IsValidDomain checks if the given string is a valid domain name.
func IsValidDomain(domain string) bool {
	domainRegex := regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)
	if !domainRegex.MatchString(domain) {
		return false
	}
	_, err := net.LookupHost(domain)
	return err == nil
}

// IsValidMACAddress checks if the given string is a valid MAC address.
func IsValidMACAddress(mac string) bool {
	macRegex := regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
	return macRegex.MatchString(mac)
}

// IsValidBase64 checks if the given string is a valid Base64 encoded string.
func IsValidBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// IsValidJSON checks if the given string is a valid JSON.
func IsValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// IsValidTime checks if the given string is a valid time in the specified format.
func IsValidTime(s string, format string, timezone string) bool {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false
	}
	_, err = time.ParseInLocation(format, s, loc)
	return err == nil
}