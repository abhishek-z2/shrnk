package shortcode

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func Encode(id uint64) string {
	if id == 0 {
		return "0"
	}

	digits := []byte{}

	for id > 0 {
		rem := id % 62
		digits = append(digits, alphabet[rem])
		id /= 62
	}

	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
