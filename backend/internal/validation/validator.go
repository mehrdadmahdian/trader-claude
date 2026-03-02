package validation

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("safe_string", validateSafeString)
	validate.RegisterValidation("symbol", validateSymbol)
	validate.RegisterValidation("timeframe", validateTimeframe)
	validate.RegisterValidation("market", validateMarket)
}

func Validate(s interface{}) error {
	return validate.Struct(s)
}

func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

var symbolRegex = regexp.MustCompile(`^[A-Z0-9]{1,10}(/[A-Z0-9]{1,10})?$`)

var validTimeframes = map[string]bool{
	"1m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "4h": true, "1d": true, "1w": true,
}

var validMarkets = map[string]bool{
	"crypto": true, "stock": true, "forex": true,
}

func validateSafeString(fl validator.FieldLevel) bool {
	s := strings.ToLower(fl.Field().String())
	dangerous := []string{"<script", "javascript:", "\x00", "'; drop", "\" or 1=1", "union select"}
	for _, d := range dangerous {
		if strings.Contains(s, d) {
			return false
		}
	}
	return true
}

func validateSymbol(fl validator.FieldLevel) bool {
	return symbolRegex.MatchString(fl.Field().String())
}

func validateTimeframe(fl validator.FieldLevel) bool {
	return validTimeframes[fl.Field().String()]
}

func validateMarket(fl validator.FieldLevel) bool {
	return validMarkets[fl.Field().String()]
}
