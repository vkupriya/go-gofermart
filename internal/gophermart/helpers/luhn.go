package helpers

func ValidOrder(number int64) bool {
	const ten = 10
	return (number%ten+checksumLuhn(number/ten))%ten == 0
}

func checksumLuhn(number int64) int64 {
	const (
		ten  = 10
		two  = 2
		nine = 9
	)
	var luhn int64

	for i := 0; number > 0; i++ {
		cur := number % ten

		if i%2 == 0 { // even
			cur *= two
			if cur > nine {
				cur = cur%ten + cur/ten
			}
		}

		luhn += cur
		number /= 10
	}
	return luhn % ten
}
